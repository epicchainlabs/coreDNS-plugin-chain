package contract

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neofs-contract/nns"
)

type Contract struct {
	client       *rpcclient.Client
	invoker      *invoker.Invoker
	contractHash util.Uint160
	nnsDomain    string
}

type Params struct {
	Endpoint     string
	ContractHash util.Uint160
	Domain       string
}

type Record struct {
	Name string
	Type nns.RecordType
	Data string
}

const dot = "."

func NewContract(ctx context.Context, prm *Params) (*Contract, error) {
	cli, err := rpcclient.New(ctx, prm.Endpoint, rpcclient.Options{})
	if err != nil {
		return nil, err
	}
	if err = cli.Init(); err != nil {
		return nil, err
	}

	if prm.ContractHash.Equals(util.Uint160{}) {
		cs, err := cli.GetContractStateByID(1)
		if err != nil {
			return nil, fmt.Errorf("get contract by id 1: %w", err)
		}
		prm.ContractHash = cs.Hash
	} else {
		if _, err = cli.GetContractStateByHash(prm.ContractHash); err != nil {
			return nil, fmt.Errorf("get contract '%s': %w", prm.ContractHash.StringLE(), err)
		}
	}

	return &Contract{
		client:       cli,
		invoker:      invoker.New(cli, nil),
		contractHash: prm.ContractHash,
		nnsDomain:    strings.Trim(prm.Domain, dot),
	}, nil
}

func (c Contract) Hash() util.Uint160 {
	return c.contractHash
}

func (c *Contract) Resolve(name string, nnsType nns.RecordType) ([]string, error) {
	item, err := unwrap.Item(c.invoker.Call(c.contractHash, "resolve", name, int64(nnsType)))
	if err != nil {
		return nil, err
	}

	var res []string

	if _, ok := item.(stackitem.Null); ok {
		return res, nil
	}

	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return nil, errors.New("invalid cast to stack item slice")
	}
	for i := range arr {
		bs, err := arr[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("convert array item to byte slice: %w", err)
		}

		res = append(res, string(bs))
	}

	return res, nil
}

func (c *Contract) GetAllRecords(name string) ([]Record, error) {
	sessionID, iterator, err := unwrap.SessionIterator(c.invoker.Call(c.contractHash, "getAllRecords", name))
	if err != nil {
		return nil, err
	}

	var records []Record
	var shouldStop bool
	batchSize := 50

	for !shouldStop {
		recordsBatchItems, err := c.invoker.TraverseIterator(sessionID, &iterator, batchSize)
		if err != nil {
			return nil, err
		}

		recordsBatch, err := getRecordsByItems(recordsBatchItems)
		if err != nil {
			return nil, err
		}

		records = append(records, recordsBatch...)
		shouldStop = len(recordsBatch) < batchSize
	}

	return records, nil
}

func (c *Contract) GetRecords(name string, nnsType nns.RecordType) ([]string, error) {
	res, err := unwrap.ArrayOfBytes(c.invoker.Call(c.contractHash, "getRecords", name, int64(nnsType)))
	if err != nil {
		return nil, err
	}

	records := make([]string, len(res))
	for i, rec := range res {
		records[i] = string(rec)
	}

	return records, nil
}

func (c Contract) PrepareName(name, dnsDomain string) string {
	name = strings.TrimSuffix(name, dot)
	if c.nnsDomain != "" {
		name = strings.TrimSuffix(strings.TrimSuffix(name, dnsDomain), dot)
		if name != "" {
			name += dot
		}
		name += c.nnsDomain
	}
	return name
}

func getRecordsByItems(items []stackitem.Item) ([]Record, error) {
	res := make([]Record, len(items))
	for i, item := range items {
		structArr, ok := item.Value().([]stackitem.Item)
		if !ok {
			return nil, errors.New("bad conversion")
		}
		if len(structArr) != 4 {
			return nil, errors.New("invalid response struct")
		}

		nameBytes, err := structArr[0].TryBytes()
		if err != nil {
			return nil, err
		}
		integer, err := structArr[1].TryInteger()
		if err != nil {
			return nil, err
		}
		typeBytes := integer.Bytes()
		if len(typeBytes) != 1 {
			return nil, errors.New("invalid nns type")
		}

		dataBytes, err := structArr[2].TryBytes()
		if err != nil {
			return nil, err
		}

		res[i] = Record{
			Name: string(nameBytes),
			Type: nns.RecordType(typeBytes[0]),
			Data: string(dataBytes),
		}
	}

	return res, nil
}
