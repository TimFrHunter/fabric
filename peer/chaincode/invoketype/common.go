package invoketype

import pb "github.com/hyperledger/fabric/protos/peer"

// EndorsementInfo structure save in file
type EndorsementInfo struct {
	ProposalResponses []*pb.ProposalResponse `json:"proposal_responses"`
	Proposal          *pb.Proposal           `json:"proposal"`
	//TxID              string                 `json:"txid"`
}

var filePath string = "/tmp/"

// FileName file name
func FileName(txid string) string {
	return filePath + "endorsementInfo_" + txid + ".json"
}
