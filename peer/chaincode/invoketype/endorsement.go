package invoketype

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

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
	uid string,
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

	prop, txid, err := putils.CreateChaincodeProposalWithTxIDAndTransient(pcommon.HeaderType_ENDORSER_TRANSACTION, cID, invocation, creator, "", tMap)
	if err != nil {
		logger.Errorf("Erreur-1 %+v", err)
		return errors.WithMessage(err, fmt.Sprintf("error creating proposal for %s", funcName))
	}

	signedProp, err := putils.GetSignedProposal(prop, signer)
	if err != nil {
		logger.Errorf("Erreur0 %+v", err)
		return errors.WithMessage(err, fmt.Sprintf("error creating signed proposal for %s", funcName))
	}

	// send proposals to endorsers
	var responses []*pb.ProposalResponse
	for _, endorser := range endorserClients {
		proposalResp, err := endorser.ProcessProposal(context.Background(), signedProp)
		if err != nil {
			logger.Errorf("Erreur1 %+v", err)
			return errors.WithMessage(err, fmt.Sprintf("error endorsing %s", funcName))
		}
		responses = append(responses, proposalResp)

		logger.Infof("%+v", responses)

	}
	logger.Infof("%v", reflect.TypeOf(signer).String())
	logger.Infof("Signer: %+v", signer)
	idemixSignerIdentity, ok := signer.(*msp.IdemixSigningIdentity)
	if ok != true {
		logger.Info("Erreur conversion interface SigningIdentity vers structure IdemixSigningIdentity")
	}
	seralizedIdexmiIdentity, err := idemixSignerIdentity.Serialize()
	if err != nil {
		logger.Infof("Erreur serialization idemix %+v", err)
	}
	logger.Infof("serializadIdemixIdentity: %v", seralizedIdexmiIdentity)
	// identityIndentifier := idemixSignerIdentity.Idemixidentity
	// idemixMSP := identityIndentifier.GetIdemixMsp()
	// identity, err := idemixMSP.DeserializeIdentity(seralizedIdexmiIdentity)
	// if err != nil {
	// 	logger.Infof("Erreur unserialization idemix %+v", err)
	// }
	// logger.Infof("UnSerializadIdemixIdentity: %v", identity)
	// idemixIdentity, ok := identity.(*msp.Idemixidentity)
	// if ok != true {
	// 	logger.Infof("Conversion identity to idemixIndentity failed")
	// }
	// idemixSignedIdentity, err := idemixIdentity.GetIdemixMsp().GetDefaultSigningIdentity()
	// if err != nil {
	// 	logger.Infof("Erreur get idemix Signed identity idemix %+v", err)
	// }
	// logger.Infof("singe didientity test: %v", idemixSignedIdentity)

	endorsementInfo := EndorsementInfo{responses, prop, txid, seralizedIdexmiIdentity}
	//logger.Infof("Apres ajout dans struct global : %+v", endorsementInfo)
	//logger.Infof("Apres ajout dans struct global sans + : %v", endorsementInfo)
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.Encode(endorsementInfo)
	fileName := FileName(uid)
	file, err := os.Create(fileName)
	if err != nil {
		return errors.New("File creation error")
	}
	defer file.Close()
	_, err = io.Copy(file, buf)
	if err != nil {
		return errors.New("File copy infos error")
	}
	logger.Infof("------Endorsement DONE-------")
	return nil
}
