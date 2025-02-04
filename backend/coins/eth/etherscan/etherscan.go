package etherscan

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/digitalbitbox/bitbox-wallet-app/backend/accounts"
	"github.com/digitalbitbox/bitbox-wallet-app/backend/coins/coin"
	ethtypes "github.com/digitalbitbox/bitbox-wallet-app/backend/coins/eth/types"
	"github.com/digitalbitbox/bitbox-wallet-app/util/errp"
	"github.com/digitalbitbox/bitbox-wallet-app/util/locker"
	"github.com/ethereum/go-ethereum/common"
)

// etherscan rate limits to one request per 0.2 seconds.
var callInterval = 210 * time.Millisecond

// EtherScan is a rate-limited etherscan api client. See https://etherscan.io/apis.
type EtherScan struct {
	url         string
	rateLimiter <-chan time.Time
	lock        locker.Locker
}

// NewEtherScan creates a new instance of EtherScan.
func NewEtherScan(url string) *EtherScan {
	return &EtherScan{
		url:         url,
		rateLimiter: time.After(0), // 0 so the first call does not wait.
	}
}

func (etherScan *EtherScan) call(params url.Values, result interface{}) error {
	defer etherScan.lock.Lock()()
	<-etherScan.rateLimiter
	defer func() {
		etherScan.rateLimiter = time.After(callInterval)
	}()

	response, err := http.Get(etherScan.url + "?" + params.Encode())
	if err != nil {
		return errp.WithStack(err)
	}
	defer func() { _ = response.Body.Close() }()
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return errp.WithStack(err)
	}
	return nil
}

type jsonBigInt big.Int

func (jsBigInt jsonBigInt) BigInt() *big.Int {
	bigInt := big.Int(jsBigInt)
	return &bigInt
}

// UnmarshalJSON implements json.Unmarshaler.
func (jsBigInt *jsonBigInt) UnmarshalJSON(jsonBytes []byte) error {
	var numberString string
	if err := json.Unmarshal(jsonBytes, &numberString); err != nil {
		return errp.WithStack(err)
	}
	bigInt, ok := new(big.Int).SetString(numberString, 10)
	if !ok {
		return errp.Newf("failed to parse %s", numberString)
	}
	*jsBigInt = jsonBigInt(*bigInt)
	return nil
}

type timestamp time.Time

// UnmarshalJSON implements json.Unmarshaler.
func (t *timestamp) UnmarshalJSON(jsonBytes []byte) error {
	var timestampString string
	if err := json.Unmarshal(jsonBytes, &timestampString); err != nil {
		return errp.WithStack(err)
	}
	timestampInt, err := strconv.ParseInt(timestampString, 10, 64)
	if err != nil {
		return errp.WithStack(err)
	}
	*t = timestamp(time.Unix(timestampInt, 0))
	return nil
}

type jsonTransaction struct {
	GasUsed       jsonBigInt     `json:"gasUsed"`
	GasPrice      jsonBigInt     `json:"gasPrice"`
	Hash          common.Hash    `json:"hash"`
	Timestamp     timestamp      `json:"timeStamp"`
	Confirmations jsonBigInt     `json:"confirmations"`
	From          common.Address `json:"from"`

	// One of them is an empty string / nil, the other is an address.
	ToAsString              string `json:"to"`
	to                      *common.Address
	ContractAddressAsString string `json:"contractAddress"`
	contractAddress         *common.Address

	Value jsonBigInt `json:"value"`
}

// Transaction implemements accounts.Transaction (TODO).
type Transaction struct {
	jsonTransaction jsonTransaction
	txType          accounts.TxType
}

// assertion because not implementing the interface fails silently.
var _ ethtypes.EthereumTransaction = &Transaction{}

// UnmarshalJSON implements json.Unmarshaler.
func (tx *Transaction) UnmarshalJSON(jsonBytes []byte) error {
	if err := json.Unmarshal(jsonBytes, &tx.jsonTransaction); err != nil {
		return errp.WithStack(err)
	}
	switch {
	case tx.jsonTransaction.ToAsString != "":
		if !common.IsHexAddress(tx.jsonTransaction.ToAsString) {
			return errp.Newf("eth address expected, got %s", tx.jsonTransaction.ToAsString)
		}
		addr := common.HexToAddress(tx.jsonTransaction.ToAsString)
		tx.jsonTransaction.to = &addr
	case tx.jsonTransaction.ContractAddressAsString != "":
		if !common.IsHexAddress(tx.jsonTransaction.ContractAddressAsString) {
			return errp.Newf("eth address expected, got %s", tx.jsonTransaction.ContractAddressAsString)
		}
		addr := common.HexToAddress(tx.jsonTransaction.ContractAddressAsString)
		tx.jsonTransaction.contractAddress = &addr
	default:
		return errp.New("Need one of: to, contractAddress")
	}
	return nil
}

// Fee implements accounts.Transaction.
func (tx *Transaction) Fee() *coin.Amount {
	fee := new(big.Int).Mul(tx.jsonTransaction.GasUsed.BigInt(), tx.jsonTransaction.GasPrice.BigInt())
	amount := coin.NewAmount(fee)
	return &amount
}

// Timestamp implements accounts.Transaction.
func (tx *Transaction) Timestamp() *time.Time {
	t := time.Time(tx.jsonTransaction.Timestamp)
	return &t
}

// ID implements accounts.Transaction.
func (tx *Transaction) ID() string {
	return tx.jsonTransaction.Hash.Hex()
}

// NumConfirmations implements accounts.Transaction.
func (tx *Transaction) NumConfirmations() int {
	return int(tx.jsonTransaction.Confirmations.BigInt().Int64())
}

// Type implements accounts.Transaction.
func (tx *Transaction) Type() accounts.TxType {
	return tx.txType
}

// Amount implements accounts.Transaction.
func (tx *Transaction) Amount() coin.Amount {
	return coin.NewAmount(tx.jsonTransaction.Value.BigInt())
}

// Addresses implements accounts.Transaction.
func (tx *Transaction) Addresses() []accounts.AddressAndAmount {
	address := ""
	if tx.jsonTransaction.to != nil {
		address = tx.jsonTransaction.to.Hex()
	} else if tx.jsonTransaction.contractAddress != nil {
		address = tx.jsonTransaction.contractAddress.Hex()
	}
	return []accounts.AddressAndAmount{{
		Address: address,
		Amount:  tx.Amount(),
	}}
}

// Gas implements ethtypes.EthereumTransaction.
func (tx *Transaction) Gas() uint64 {
	if !tx.jsonTransaction.GasUsed.BigInt().IsInt64() {
		panic("gas must be int64")
	}
	return uint64(tx.jsonTransaction.GasUsed.BigInt().Int64())
}

// prepareTransactions casts to []accounts.Transactions and removes duplicate entries. Duplicate entries
// appear in the etherscan result if the recipient and sender are the same. It also sets the
// transaction type (send, receive, send to self) based on the account address.
func prepareTransactions(
	transactions []*Transaction, address common.Address) ([]accounts.Transaction, error) {
	seen := map[string]struct{}{}
	castTransactions := []accounts.Transaction{}
	ours := address.Hex()
	for _, transaction := range transactions {
		if _, ok := seen[transaction.ID()]; ok {
			continue
		}
		seen[transaction.ID()] = struct{}{}

		from := transaction.jsonTransaction.From.Hex()
		var to string
		switch {
		case transaction.jsonTransaction.to != nil:
			to = transaction.jsonTransaction.to.Hex()
		case transaction.jsonTransaction.contractAddress != nil:
			to = transaction.jsonTransaction.contractAddress.Hex()
		default:
			return nil, errp.New("must have either to address or contract address")
		}
		if ours != from && ours != to {
			return nil, errp.New("transaction does not belong to our account")
		}
		switch {
		case ours == from && ours == to:
			transaction.txType = accounts.TxTypeSendSelf
		case ours == from:
			transaction.txType = accounts.TxTypeSend
		default:
			transaction.txType = accounts.TxTypeReceive
		}
		castTransactions = append(castTransactions, transaction)
	}
	return castTransactions, nil
}

// Transactions queries EtherScan for transactions for the given account, until endBlock.
func (etherScan *EtherScan) Transactions(address common.Address, endBlock *big.Int) (
	[]accounts.Transaction, error) {
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("startblock", "0")
	params.Set("tag", "latest")
	params.Set("sort", "desc") // desc by block number

	params.Set("endblock", endBlock.Text(10))
	params.Set("address", address.Hex())

	result := struct {
		Result []*Transaction
	}{}
	if err := etherScan.call(params, &result); err != nil {
		return nil, err
	}

	return prepareTransactions(result.Result, address)
}
