package invoketype

import (
	pb "github.com/hyperledger/fabric/protos/peer"
)

// EndorsementInfo structure save in file
type EndorsementInfo struct {
	ProposalResponses []*pb.ProposalResponse `json:"proposal_responses"`
	Proposal          *pb.Proposal           `json:"proposal"`
	Txid              string                 `json:"txid"`
	IdemixSigner      []byte                 `json:"idemix_signer"`
}

// EndorsementInfoBis structure to read
// type EndorsementInfoBis struct {
// 	ProposalResponses []*pb.ProposalResponse `json:"proposal_responses"`
// 	Proposal          *pb.Proposal           `json:"proposal"`
// 	Txid              string                 `json:"txid"`
// 	IdemixSigner      json.RawMessage
// }

// type idemixSigningIdentity struct {
// 	*idemixidentity
// 	Cred         []byte
// 	UserKey      bccsp.Key
// 	NymKey       bccsp.Key
// 	enrollmentId string
// }

// type idemixidentity struct {
// 	NymPublicKey bccsp.Key
// 	//msp          *idemixmsp
// 	//id           *IdentityIdentifier
// 	Role *m.MSPRole
// 	OU   *m.OrganizationUnit
// 	// associationProof contains cryptographic proof that this identity
// 	// belongs to the MSP id.msp, i.e., it proves that the pseudonym
// 	// is constructed from a secret key on which the CA issued a credential.
// 	associationProof []byte
// }

var filePath = "/tmp/"

// FileName file name
func FileName(uid string) string {
	return filePath + uid + ".json"
}
