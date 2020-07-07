package chaincode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/msp"
	pcommon "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	putils "github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

func endorsement(
	spec *pb.ChaincodeSpec,
	cID string,
	txID string,
	signer msp.SigningIdentity,
	endorserClients []pb.EndorserClient,
) error {
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

	prop, txID, err := putils.CreateChaincodeProposalWithTxIDAndTransient(pcommon.HeaderType_ENDORSER_TRANSACTION, cID, invocation, creator, txID, tMap)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error creating proposal for %s", funcName))
	}

	signedProp, err := putils.GetSignedProposal(prop, signer)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error creating signed proposal for %s", funcName))
	}
	var responses []*pb.ProposalResponse
	for _, endorser := range endorserClients {
		proposalResp, err := endorser.ProcessProposal(context.Background(), signedProp)
		if err != nil {
			return errors.WithMessage(err, fmt.Sprintf("error endorsing %s", funcName))
		}
		responses = append(responses, proposalResp)
	}

	if len(responses) == 0 {
		// this should only happen if some new code has introduced a bug
		return errors.New("no proposal responses received - this might indicate a bug")
	}
	// all responses will be checked when the signed transaction is created.
	// for now, just set this so we check the first response's status
	proposalResp := responses[0]

	if proposalResp != nil {
		if proposalResp.Response.Status >= shim.ERRORTHRESHOLD {
			return nil
		}
		// assemble a signed transaction (it's an Envelope message)
		env, err := putils.CreateSignedTx(prop, signer, responses...)
		if err != nil {
			return errors.WithMessage(err, "could not assemble transaction")
		}
		buf := &bytes.Buffer{}
		encoder := json.NewEncoder(buf)
		encoder.Encode(env)
		fileName := "/tmp/" + uid + ".json" //FileName(uid)
		file, err := os.Create(fileName)
		if err != nil {
			return errors.New("File creation error")
		}
		defer file.Close()
		_, err = io.Copy(file, buf)
		if err != nil {
			return errors.New("File copy infos error")
		}
	}
	return nil
}
