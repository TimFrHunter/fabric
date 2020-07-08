package chaincode

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/common/api"
	"github.com/pkg/errors"
)

func validation(
	deliverClients []api.PeerDeliverClient,
	peerAddresses []string,
	certificate tls.Certificate,
	channelID string,
	bc common.BroadcastClient,
) error {

	fileName := "/tmp/" + uid + ".json"
	fileBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.WithMessage(err, "could not open file")
	}
	var endorsementFileInfoStruct endorsementFileInfo
	err = json.Unmarshal(fileBytes, &endorsementFileInfoStruct)
	if err != nil {
		return errors.WithMessage(err, "could not decode file")
	}
	txid := endorsementFileInfoStruct.Txid
	env := endorsementFileInfoStruct.Envelope
	err = os.Remove(fileName)
	if err != nil {
		return err
	}

	//Wait for event
	var dg *deliverGroup
	var ctx context.Context
	var cancelFunc context.CancelFunc
	ctx, cancelFunc = context.WithTimeout(context.Background(), waitForEventTimeout)
	defer cancelFunc()

	dg = newDeliverGroup(deliverClients, peerAddresses, certificate, channelID, txid)
	// connect to deliver service on all peers
	err = dg.Connect(ctx)
	if err != nil {
		return err
	}

	// Send the envelope for ordering
	if err := bc.Send(env); err != nil {
		return errors.WithMessage(err, "error sending transaction for invoke")
	}

	if dg != nil && ctx != nil {
		// wait for event that contains the txid from all peers
		err = dg.Wait(ctx)
		if err != nil {
			return err
		}
	}
	return nil

}
