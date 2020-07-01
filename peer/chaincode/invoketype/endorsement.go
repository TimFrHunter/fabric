package invoketype

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/msp"
	pcommon "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

var logger = flogging.MustGetLogger("chaincodeEndorsementCMD")

// Endorsement returns an array with endorsers responses
func Endorsement(
	transient string,
	spec *pb.ChaincodeSpec,
	cID string,
	txID string,
	invoke bool,
	signer msp.SigningIdentity,
	certificate tls.Certificate,
	endorserClients []pb.EndorserClient,
) error {
	logger.Infof("---- START ENDORSEMENT------")
	// Build the ChaincodeInvocationSpec message
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	creator, err := signer.Serialize()
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error serializing identity for %s", signer.GetIdentifier()))
	}

	funcName := "invoke"

	// extract the transient field if it exists
	var tMap map[string][]byte
	if transient != "" {
		if err := json.Unmarshal([]byte(transient), &tMap); err != nil {
			return errors.Wrap(err, "error parsing transient string")
		}
	}

	prop, txid, err := putils.CreateChaincodeProposalWithTxIDAndTransient(pcommon.HeaderType_ENDORSER_TRANSACTION, cID, invocation, creator, txID, tMap)
	if err != nil {
		logger.Infof("Erreur-1 %+v", err)
		return errors.WithMessage(err, fmt.Sprintf("error creating proposal for %s", funcName))
	}

	logger.Infof("txid:%s", txid)
	logger.Infof("txID:%s", txID)
	signedProp, err := putils.GetSignedProposal(prop, signer)
	if err != nil {
		logger.Infof("Erreur0 %+v", err)
		return errors.WithMessage(err, fmt.Sprintf("error creating signed proposal for %s", funcName))
	}

	// send proposals to endorsers
	var responses []*pb.ProposalResponse
	for _, endorser := range endorserClients {
		proposalResp, err := endorser.ProcessProposal(context.Background(), signedProp)
		if err != nil {
			logger.Infof("Erreur1 %+v", err)
			return errors.WithMessage(err, fmt.Sprintf("error endorsing %s", funcName))
		}
		responses = append(responses, proposalResp)

		logger.Infof("%+v", responses)

	}

	endorsementInfo := EndorsementInfo{responses, prop}
	endorsementInfoJSON, err := json.Marshal(endorsementInfo)
	if err != nil {
		logger.Infof("Erreur2 %+v", err)
	}
	fileName := FileName(txid)
	err = ioutil.WriteFile(fileName, endorsementInfoJSON, 0644)
	if err != nil {
		logger.Infof("Erreur3 %+v", err)
	}
	logger.Infof("------Endorsement DONE-------")
	return nil
}
