package chaincode

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/common/api"
	cm "github.com/hyperledger/fabric/protos/common"
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
	var env *cm.Envelope
	err = json.Unmarshal(fileBytes, &env)
	if err != nil {
		return nil, errors.WithMessage(err, "could not decode file")
	}
	// file, err := os.OpenFile(fileName, os.O_RDWR, 0755)
	// if err != nil {
	// 	return nil, errors.WithMessage(err, "could not open file")
	// }
	// defer file.Close()
	// dec := json.NewDecoder(file)
	// buf := &bytes.Buffer{}
	// err = dec.Decode(buf)
	// if err != nil {
	// 	return nil, errors.WithMessage(err, "could not decode file")
	// }
	// logger.Infof("Buf ? : %+v", fileBytes)
	// logger.Infof("Env ? : %+v", env)
	// if 1 == 1 {
	// 	return nil, errors.New("Coupure avant de tout faire sauter ;)")
	// }
	// return nil, errors.New("Coupure avant de tout faire sauter ;)")

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
		return proposalResp, errors.WithMessage(err, fmt.Sprintf("error sending transaction for %s", funcName))
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
