package invoketype

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/localmsp"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/msp"
	ccapi "github.com/hyperledger/fabric/peer/chaincode/api"
	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/common/api"
	pcommon "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

type deliverGroup struct {
	Clients     []*deliverClient
	Certificate tls.Certificate
	ChannelID   string
	TxID        string
	mutex       sync.Mutex
	Error       error
	wg          sync.WaitGroup
}

type deliverClient struct {
	Client     api.PeerDeliverClient
	Connection ccapi.Deliver
	Address    string
}

// Validation for ordering services
func Validation(
	txid string,
	signer msp.SigningIdentity, // nope
	deliverClients []api.PeerDeliverClient, // nope
	peerAddresses []string, // nope
	certificate tls.Certificate, // nope
	channelID string, // nope
	bc common.BroadcastClient, //nope
) (*pb.ProposalResponse, error) {

	endorsementInfo := EndorsementInfo{}
	fileName := FileName(txid)
	endorsementInfoJSON, _ := ioutil.ReadFile(fileName)
	_ = json.Unmarshal([]byte(endorsementInfoJSON), &endorsementInfo)
	responses := endorsementInfo.ProposalResponses
	prop := endorsementInfo.Proposal

	if len(responses) == 0 {
		// this should only happen if some new code has introduced a bug
		return nil, errors.New("no proposal responses received - this might indicate a bug")
	}
	// all responses will be checked when the signed transaction is created.
	// for now, just set this so we check the first response's status
	proposalResp := responses[0]

	if proposalResp != nil {
		if proposalResp.Response.Status >= shim.ERRORTHRESHOLD {
			return proposalResp, nil
		}
		var waitForEventTimeout time.Duration = 30 * time.Second
		// assemble a signed transaction (it's an Envelope message)
		env, err := putils.CreateSignedTx(prop, signer, responses...)
		if err != nil {
			return proposalResp, errors.WithMessage(err, "could not assemble transaction")
		}
		var dg *deliverGroup
		var ctx context.Context
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithTimeout(context.Background(), waitForEventTimeout) // doit etre injecté depuis autre paquet
		defer cancelFunc()

		dg = newDeliverGroup(deliverClients, peerAddresses, certificate, channelID, txid)
		// connect to deliver service on all peers
		err = dg.Connect(ctx)
		if err != nil {
			return nil, err
		}

		// send the envelope for ordering
		if err = bc.Send(env); err != nil {
			return proposalResp, errors.WithMessage(err, fmt.Sprintf("error sending transaction for invoke"))
		}

		if dg != nil && ctx != nil {
			// wait for event that contains the txid from all peers
			err = dg.Wait(ctx)
			if err != nil {
				return nil, err
			}
		}
	}
	logger.Infof("-----Validation DONE-----")
	return proposalResp, nil
}

func newDeliverGroup(deliverClients []api.PeerDeliverClient, peerAddresses []string, certificate tls.Certificate, channelID string, txid string) *deliverGroup {
	clients := make([]*deliverClient, len(deliverClients))
	for i, client := range deliverClients {
		dc := &deliverClient{
			Client:  client,
			Address: peerAddresses[i],
		}
		clients[i] = dc
	}

	dg := &deliverGroup{
		Clients:     clients,
		Certificate: certificate,
		ChannelID:   channelID,
		TxID:        txid,
	}

	return dg
}

// Connect waits for all deliver clients in the group to connect to
// the peer's deliver service, receive an error, or for the context
// to timeout. An error will be returned whenever even a single
// deliver client fails to connect to its peer
func (dg *deliverGroup) Connect(ctx context.Context) error {
	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientConnect(ctx, client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			err := errors.WithMessage(dg.Error, "failed to connect to deliver on all peers")
			return err
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for connection to deliver on all peers")
		return err
	}

	return nil
}

// ClientConnect sends a deliver seek info envelope using the
// provided deliver client, setting the deliverGroup's Error
// field upon any error
func (dg *deliverGroup) ClientConnect(ctx context.Context, dc *deliverClient) {
	defer dg.wg.Done()
	df, err := dc.Client.DeliverFiltered(ctx)
	if err != nil {
		err = errors.WithMessage(err, fmt.Sprintf("error connecting to deliver filtered at %s", dc.Address))
		dg.setError(err)
		return
	}
	defer df.CloseSend()
	dc.Connection = df

	envelope := createDeliverEnvelope(dg.ChannelID, dg.Certificate)
	err = df.Send(envelope)
	if err != nil {
		err = errors.WithMessage(err, fmt.Sprintf("error sending deliver seek info envelope to %s", dc.Address))
		dg.setError(err)
		return
	}
}

// Wait waits for all deliver client connections in the group to
// either receive a block with the txid, an error, or for the
// context to timeout
func (dg *deliverGroup) Wait(ctx context.Context) error {
	if len(dg.Clients) == 0 {
		return nil
	}

	dg.wg.Add(len(dg.Clients))
	for _, client := range dg.Clients {
		go dg.ClientWait(client)
	}
	readyCh := make(chan struct{})
	go dg.WaitForWG(readyCh)

	select {
	case <-readyCh:
		if dg.Error != nil {
			err := errors.WithMessage(dg.Error, "failed to receive txid on all peers")
			return err
		}
	case <-ctx.Done():
		err := errors.New("timed out waiting for txid on all peers")
		return err
	}

	return nil
}

// ClientWait waits for the specified deliver client to receive
// a block event with the requested txid
func (dg *deliverGroup) ClientWait(dc *deliverClient) {
	defer dg.wg.Done()
	for {
		resp, err := dc.Connection.Recv()
		if err != nil {
			err = errors.WithMessage(err, fmt.Sprintf("error receiving from deliver filtered at %s", dc.Address))
			dg.setError(err)
			return
		}
		switch r := resp.Type.(type) {
		case *pb.DeliverResponse_FilteredBlock:
			filteredTransactions := r.FilteredBlock.FilteredTransactions
			for _, tx := range filteredTransactions {
				if tx.Txid == dg.TxID {
					logger.Infof("txid [%s] committed with status (%s) at %s", dg.TxID, tx.TxValidationCode, dc.Address)
					return
				}
			}
		case *pb.DeliverResponse_Status:
			err = errors.Errorf("deliver completed with status (%s) before txid received", r.Status)
			dg.setError(err)
			return
		default:
			err = errors.Errorf("received unexpected response type (%T) from %s", r, dc.Address)
			dg.setError(err)
			return
		}
	}
}

// WaitForWG waits for the deliverGroup's wait group and closes
// the channel when ready
func (dg *deliverGroup) WaitForWG(readyCh chan struct{}) {
	dg.wg.Wait()
	close(readyCh)
}

// setError serializes an error for the deliverGroup
func (dg *deliverGroup) setError(err error) {
	dg.mutex.Lock()
	dg.Error = err
	dg.mutex.Unlock()
}

func createDeliverEnvelope(channelID string, certificate tls.Certificate) *pcommon.Envelope {
	var tlsCertHash []byte
	// check for client certificate and create hash if present
	if len(certificate.Certificate) > 0 {
		tlsCertHash = util.ComputeSHA256(certificate.Certificate[0])
	}

	start := &ab.SeekPosition{
		Type: &ab.SeekPosition_Newest{
			Newest: &ab.SeekNewest{},
		},
	}

	stop := &ab.SeekPosition{
		Type: &ab.SeekPosition_Specified{
			Specified: &ab.SeekSpecified{
				Number: math.MaxUint64,
			},
		},
	}

	seekInfo := &ab.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: ab.SeekInfo_BLOCK_UNTIL_READY,
	}

	env, err := putils.CreateSignedEnvelopeWithTLSBinding(
		pcommon.HeaderType_DELIVER_SEEK_INFO, channelID, localmsp.NewSigner(),
		seekInfo, int32(0), uint64(0), tlsCertHash)
	if err != nil {
		logger.Errorf("Error signing envelope: %s", err)
		return nil
	}

	return env
}
