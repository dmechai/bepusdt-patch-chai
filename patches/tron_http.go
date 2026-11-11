package task

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/v03413/bepusdt/app/conf"
	"github.com/v03413/bepusdt/app/model"
	"github.com/v03413/tronprotocol/api"
	"github.com/v03413/tronprotocol/core"
	"google.golang.org/protobuf/types/known/anypb"
)

var tronHTTPClient = &http.Client{Timeout: 30 * time.Second}

func tronEndpointUsesHTTP(endpoint string) bool {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))

	return strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")
}

func (t *tron) fetchNowBlock(ctx context.Context) (*api.BlockExtention, error) {
	if !tronEndpointUsesHTTP(model.Endpoint(conf.Tron)) {
		conn, err := t.client()
		if err != nil {
			return nil, err
		}

		return api.NewWalletClient(conn).GetNowBlock2(ctx, nil)
	}

	raw, err := t.tronHTTPPost(ctx, "/wallet/getnowblock", map[string]any{})
	if err != nil {
		return nil, err
	}

	return tronHTTPToBlock(raw)
}

func (t *tron) fetchBlockByNum(ctx context.Context, num int) (*api.BlockExtention, error) {
	if !tronEndpointUsesHTTP(model.Endpoint(conf.Tron)) {
		conn, err := t.client()
		if err != nil {
			return nil, err
		}

		return api.NewWalletClient(conn).GetBlockByNum2(ctx, &api.NumberMessage{Num: int64(num)})
	}

	raw, err := t.tronHTTPPost(ctx, "/wallet/getblockbynum", map[string]any{"num": num})
	if err != nil {
		return nil, err
	}

	block, err := tronHTTPToBlock(raw)
	if err != nil {
		return nil, err
	}
	if got := block.GetBlockHeader().GetRawData().GetNumber(); got != int64(num) {
		return nil, fmt.Errorf("tron http block number %d != %d", got, num)
	}

	return block, nil
}

func (t *tron) fetchTransactionByID(ctx context.Context, txID string) (*core.Transaction, error) {
	raw, err := t.tronHTTPPost(ctx, "/wallet/gettransactionbyid", map[string]string{"value": txID})
	if err != nil {
		return nil, err
	}

	var tx tronHTTPTransaction
	if err = json.Unmarshal(raw, &tx); err != nil {
		return nil, err
	}
	msg, _, _, err := tronHTTPToTransaction(tx)

	return msg, err
}

func (t *tron) fetchTransactionInfoByID(ctx context.Context, txID string) (*core.TransactionInfo, error) {
	raw, err := t.tronHTTPPost(ctx, "/wallet/gettransactioninfobyid", map[string]string{"value": txID})
	if err != nil {
		return nil, err
	}

	return tronHTTPToInfo(raw)
}

func (t *tron) tronHTTPPost(ctx context.Context, path string, body any) ([]byte, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(model.Endpoint(conf.Tron)), "/")
	if !tronEndpointUsesHTTP(endpoint) {
		return nil, errors.New("tron endpoint is not http")
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var lastErr error
	attempts := 1
	if path == "/wallet/getnowblock" || path == "/wallet/getblockbynum" {
		attempts = 3
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if key := tronAPIKey(); key != "" {
			req.Header.Set("TRON-PRO-API-KEY", key)
		}

		resp, err := tronHTTPClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
			} else if path == "/wallet/getnowblock" || path == "/wallet/getblockbynum" {
				if readyErr := tronHTTPBlockReady(data); readyErr != nil {
					lastErr = readyErr
				} else {
					return data, nil
				}
			} else {
				return data, nil
			}
		}
		if attempt == attempts || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
		}
	}

	return nil, fmt.Errorf("tron http %s: %w", path, lastErr)
}

func tronHTTPBlockReady(raw []byte) error {
	var probe struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	if probe.BlockHeader.RawData.Number <= 0 {
		return errors.New("block not ready")
	}

	return nil
}

func tronAPIKey() string {
	keys := model.GetTronGridApiKeys()
	if len(keys) == 0 {
		return ""
	}

	return keys[time.Now().UnixMicro()%int64(len(keys))]
}

type tronHTTPBlock struct {
	BlockHeader struct {
		RawData struct {
			Number    int64 `json:"number"`
			Timestamp int64 `json:"timestamp"`
		} `json:"raw_data"`
	} `json:"block_header"`
	Transactions []tronHTTPTransaction `json:"transactions"`
}

type tronHTTPTransaction struct {
	TxID    string `json:"txID"`
	RawData struct {
		Contract []struct {
			Type      string `json:"type"`
			Parameter struct {
				Value map[string]json.RawMessage `json:"value"`
			} `json:"parameter"`
		} `json:"contract"`
	} `json:"raw_data"`
	Ret []struct {
		ContractRet string `json:"contractRet"`
	} `json:"ret"`
}

type tronHTTPInfo struct {
	ID      string `json:"id"`
	Receipt struct {
		Result string `json:"result"`
	} `json:"receipt"`
}

func tronHTTPToBlock(raw []byte) (*api.BlockExtention, error) {
	var block tronHTTPBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, err
	}
	if block.BlockHeader.RawData.Number <= 0 {
		return nil, errors.New("tron http block missing number")
	}

	out := &api.BlockExtention{
		BlockHeader: &core.BlockHeader{RawData: &core.BlockHeaderRaw{
			Number:    block.BlockHeader.RawData.Number,
			Timestamp: block.BlockHeader.RawData.Timestamp,
		}},
	}
	for _, tx := range block.Transactions {
		msg, id, ok, err := tronHTTPToTransaction(tx)
		if err != nil {
			return nil, err
		}
		out.Transactions = append(out.Transactions, &api.TransactionExtention{
			Transaction: msg,
			Txid:        id,
			Result:      &api.Return{Result: ok},
		})
	}

	return out, nil
}

func tronHTTPToTransaction(tx tronHTTPTransaction) (*core.Transaction, []byte, bool, error) {
	var id []byte
	var err error
	if tx.TxID != "" {
		id, err = hex.DecodeString(tx.TxID)
		if err != nil {
			return nil, nil, false, fmt.Errorf("decode txID: %w", err)
		}
	}

	msg := &core.Transaction{RawData: &core.TransactionRaw{}}
	success := len(tx.Ret) == 0
	for _, ret := range tx.Ret {
		code, ok := core.Transaction_ResultContractResult_value[ret.ContractRet]
		if !ok {
			return nil, nil, false, fmt.Errorf("unknown contract result %q", ret.ContractRet)
		}
		result := core.Transaction_ResultContractResult(code)
		msg.Ret = append(msg.Ret, &core.Transaction_Result{ContractRet: result})
		if result == core.Transaction_Result_SUCCESS {
			success = true
		}
	}

	for _, contract := range tx.RawData.Contract {
		code, ok := core.Transaction_Contract_ContractType_value[contract.Type]
		if !ok {
			continue
		}
		typ := core.Transaction_Contract_ContractType(code)
		parameter, err := tronHTTPContractParameter(typ, contract.Parameter.Value)
		if err != nil {
			return nil, nil, false, err
		}
		msg.RawData.Contract = append(msg.RawData.Contract, &core.Transaction_Contract{
			Type:      typ,
			Parameter: parameter,
		})
	}

	return msg, id, success, nil
}

func tronHTTPToInfo(raw []byte) (*core.TransactionInfo, error) {
	var info tronHTTPInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	if info.Receipt.Result == "" {
		return &core.TransactionInfo{}, nil
	}
	code, ok := core.Transaction_ResultContractResult_value[info.Receipt.Result]
	if !ok {
		return nil, fmt.Errorf("unknown receipt result %q", info.Receipt.Result)
	}

	return &core.TransactionInfo{Receipt: &core.ResourceReceipt{
		Result: core.Transaction_ResultContractResult(code),
	}}, nil
}

func tronHTTPContractParameter(typ core.Transaction_Contract_ContractType, value map[string]json.RawMessage) (*anypb.Any, error) {
	switch typ {
	case core.Transaction_Contract_TransferContract:
		owner, err := rawAddress(value, "owner_address")
		if err != nil {
			return nil, err
		}
		to, err := rawAddress(value, "to_address")
		if err != nil {
			return nil, err
		}
		amount, err := rawInt64(value, "amount")
		if err != nil {
			return nil, err
		}

		return anypb.New(&core.TransferContract{OwnerAddress: owner, ToAddress: to, Amount: amount})
	case core.Transaction_Contract_TriggerSmartContract:
		owner, err := rawAddress(value, "owner_address")
		if err != nil {
			return nil, err
		}
		contract, err := rawAddress(value, "contract_address")
		if err != nil {
			return nil, err
		}
		dataHex, err := rawString(value, "data")
		if err != nil {
			return nil, err
		}
		data, err := hex.DecodeString(dataHex)
		if err != nil {
			return nil, fmt.Errorf("decode contract data: %w", err)
		}

		return anypb.New(&core.TriggerSmartContract{OwnerAddress: owner, ContractAddress: contract, Data: data})
	case core.Transaction_Contract_DelegateResourceContract:
		msg, err := resourceContract(value)
		if err != nil {
			return nil, err
		}

		return anypb.New(&core.DelegateResourceContract{
			OwnerAddress:    msg.owner,
			ReceiverAddress: msg.receiver,
			Balance:         msg.balance,
			Resource:        msg.resource,
		})
	case core.Transaction_Contract_UnDelegateResourceContract:
		msg, err := resourceContract(value)
		if err != nil {
			return nil, err
		}

		return anypb.New(&core.UnDelegateResourceContract{
			OwnerAddress:    msg.owner,
			ReceiverAddress: msg.receiver,
			Balance:         msg.balance,
			Resource:        msg.resource,
		})
	default:
		return nil, nil
	}
}

type resourceFields struct {
	owner    []byte
	receiver []byte
	balance  int64
	resource core.ResourceCode
}

func resourceContract(value map[string]json.RawMessage) (resourceFields, error) {
	owner, err := rawAddress(value, "owner_address")
	if err != nil {
		return resourceFields{}, err
	}
	receiver, err := rawAddress(value, "receiver_address")
	if err != nil {
		return resourceFields{}, err
	}
	balance, err := rawInt64(value, "balance")
	if err != nil {
		return resourceFields{}, err
	}
	resource, err := rawResource(value)
	if err != nil {
		return resourceFields{}, err
	}

	return resourceFields{owner: owner, receiver: receiver, balance: balance, resource: resource}, nil
}

func rawAddress(value map[string]json.RawMessage, key string) ([]byte, error) {
	text, err := rawString(value, key)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, fmt.Errorf("missing %s", key)
	}
	if strings.HasPrefix(text, "T") {
		raw := base58.Decode(text)
		if len(raw) < 25 {
			return nil, fmt.Errorf("invalid base58 address %s", key)
		}

		return raw[:21], nil
	}

	text = strings.TrimPrefix(strings.TrimPrefix(text, "0x"), "0X")
	raw, err := hex.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	if len(raw) == 20 {
		return append([]byte{0x41}, raw...), nil
	}
	if len(raw) != 21 {
		return nil, fmt.Errorf("invalid address length %s", key)
	}

	return raw, nil
}

func rawString(value map[string]json.RawMessage, key string) (string, error) {
	raw, ok := value[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", fmt.Errorf("decode %s: %w", key, err)
	}

	return text, nil
}

func rawInt64(value map[string]json.RawMessage, key string) (int64, error) {
	raw, ok := value[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("decode %s: %w", key, err)
	}

	return number.Int64()
}

func rawResource(value map[string]json.RawMessage) (core.ResourceCode, error) {
	text, err := rawString(value, "resource")
	if err != nil {
		return 0, err
	}
	if text == "" {
		number, err := rawInt64(value, "resource")
		if err != nil {
			return 0, err
		}

		return core.ResourceCode(number), nil
	}
	code, ok := core.ResourceCode_value[text]
	if !ok {
		return 0, fmt.Errorf("unknown resource %q", text)
	}

	return core.ResourceCode(code), nil
}
