package chaincode

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"

	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/common/api"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

func validation(
	deliverClients []api.PeerDeliverClient, // nope
	peerAddresses []string, // nope
	certificate tls.Certificate, // nope
	channelID string, // nope
	bc common.BroadcastClient, //nope
) (*pb.ProposalResponse, error) {

	fileName := "/tmp/" + uid + ".json"
	fileBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.WithMessage(err, "could not open file")
	}
	var endorsementFileInfoStruct endorsementFileInfo
	err = json.Unmarshal(fileBytes, &endorsementFileInfoStruct)
	if err != nil {
		return nil, errors.WithMessage(err, "could not decode file")
	}
	logger.Infof("Endorsement file info struct: %+v", endorsementFileInfoStruct)

	txid := endorsementFileInfoStruct.Txid
	responses := endorsementFileInfoStruct.Responses
	env := endorsementFileInfoStruct.Envelope
	proposalResp := responses[0]
	if len(responses) == 0 {
		// this should only happen if some new code has introduced a bug
		return nil, errors.New("no proposal responses received - this might indicate a bug")
	}
	waitForEvent = true
	var dg *deliverGroup
	var ctx context.Context
	if waitForEvent {
		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithTimeout(context.Background(), waitForEventTimeout)
		defer cancelFunc()

		dg = newDeliverGroup(deliverClients, peerAddresses, certificate, channelID, txid)
		// connect to deliver service on all peers
		err := dg.Connect(ctx)
		if err != nil {
			return nil, err
		}
	}

	// send the envelope for ordering
	if err := bc.Send(env); err != nil {
		return proposalResp, errors.WithMessage(err, "error sending transaction for invoke")
	}

	if dg != nil && ctx != nil {
		// wait for event that contains the txid from all peers
		err = dg.Wait(ctx)
		if err != nil {
			return nil, err
		}
	}
	return proposalResp, nil

}
