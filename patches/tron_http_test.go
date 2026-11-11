package task

import (
	"encoding/hex"
	"testing"

	"github.com/v03413/tronprotocol/core"
)

func TestTronEndpointUsesHTTP(t *testing.T) {
	if tronEndpointUsesHTTP("grpc.trongrid.io:50051") {
		t.Fatal("host:port should stay on grpc")
	}
	if !tronEndpointUsesHTTP("https://api.trongrid.io") {
		t.Fatal("https endpoint should use http wallet api")
	}
}

func TestTronHTTPBlockConversion(t *testing.T) {
	raw := []byte(`{
		"block_header":{"raw_data":{"number":87900000,"timestamp":1794373809000}},
		"transactions":[
			{
				"txID":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"raw_data":{"contract":[{"type":"TransferContract","parameter":{"value":{
					"owner_address":"41fe4ef207fd7d84fe4ef207fd7d84fe4ef207fd7d",
					"to_address":"41b35a8589b4aad741b35a8589b4aad741b35a8589",
					"amount":1500000
				}}}]},
				"ret":[{"contractRet":"SUCCESS"}]
			},
			{
				"txID":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"raw_data":{"contract":[{"type":"TriggerSmartContract","parameter":{"value":{
					"owner_address":"410746ff3ac5e76a410746ff3ac5e76a410746ff3a",
					"contract_address":"41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
					"data":"a9059cbb000000000000000000000000b35a8589b4aad741b35a8589b4aad741b35a858900000000000000000000000000000000000000000000000000000000000f4240"
				}}}]},
				"ret":[{"contractRet":"REVERT"}]
			}
		]
	}`)

	block, err := tronHTTPToBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	if block.GetBlockHeader().GetRawData().GetNumber() != 87900000 {
		t.Fatalf("number = %d", block.GetBlockHeader().GetRawData().GetNumber())
	}
	if len(block.GetTransactions()) != 2 {
		t.Fatalf("transactions = %d", len(block.GetTransactions()))
	}
	if !block.Transactions[0].GetResult().GetResult() || block.Transactions[1].GetResult().GetResult() {
		t.Fatal("unexpected transaction success flags")
	}

	var transfer core.TransferContract
	if err = block.Transactions[0].GetTransaction().GetRawData().GetContract()[0].GetParameter().UnmarshalTo(&transfer); err != nil {
		t.Fatal(err)
	}
	if transfer.GetAmount() != 1500000 || len(transfer.GetToAddress()) != 21 {
		t.Fatalf("transfer = %+v", transfer.GetAmount())
	}

	var trigger core.TriggerSmartContract
	if err = block.Transactions[1].GetTransaction().GetRawData().GetContract()[0].GetParameter().UnmarshalTo(&trigger); err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(trigger.GetContractAddress(), usdtTrc20ContractAddress) || len(trigger.GetData()) < 4 {
		t.Fatalf("trigger contract = %s", hex.EncodeToString(trigger.GetContractAddress()))
	}
}

func TestTronHTTPBlockReady(t *testing.T) {
	if err := tronHTTPBlockReady([]byte(`{"block_header":{"raw_data":{"number":87900000}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tronHTTPBlockReady([]byte(`{}`)); err == nil {
		t.Fatal("empty block should be retried")
	}
}

func TestTronHTTPInfoConversion(t *testing.T) {
	info, err := tronHTTPToInfo([]byte(`{"id":"aa","receipt":{"result":"SUCCESS"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.GetReceipt().GetResult() != core.Transaction_Result_SUCCESS {
		t.Fatalf("result = %s", info.GetReceipt().GetResult())
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
