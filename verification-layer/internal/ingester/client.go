package ingester

import (
	"context"
	"fmt"

	providers "github.com/openweb3/go-rpc-provider/provider_wrapper"
	"github.com/0gfoundation/0g-storage-client/common/blockchain"
	"github.com/0gfoundation/0g-storage-client/core"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/transfer"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/openweb3/web3go"
)

type StorageClient struct {
	w3Client      *web3go.Client
	indexerClient *indexer.Client
}

func NewStorageClient(rpcURL, privateKey, indexerURL string) (*StorageClient, error) {
	// 1. Init Web3 Client
	w3client := blockchain.MustNewWeb3(rpcURL, privateKey, providers.Option{})
	
	// 2. Init Indexer Client
	indexerClient, err := indexer.NewClient(indexerURL, indexer.IndexerClientOption{})
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer client: %w", err)
	}

	return &StorageClient{
		w3Client:      w3client,
		indexerClient: indexerClient,
	}, nil
}

// UploadFile uploads a file to 0G Storage and returns the Root Hash.
func (c *StorageClient) UploadFile(ctx context.Context, filePath string) (string, error) {
	file, err := core.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Default upload options
	opt := transfer.UploadOption{
		Tags:             hexutil.MustDecode("0x"),
		FinalityRequired: transfer.TransactionPacked, // We don't need to wait for full finality for MVP
		ExpectedReplica:  1,
		SkipTx:           false,
		FullTrusted:      true,
	}

	// Get uploader from indexer nodes
	uploader, err := c.indexerClient.NewUploaderFromIndexerNodes(ctx, file.NumSegments(), c.w3Client, opt.ExpectedReplica, nil, "max", opt.FullTrusted)
	if err != nil {
		return "", fmt.Errorf("failed to create uploader: %w", err)
	}

	fragmentSize := int64(4294967296) // Large fragment size from default configs

	// Perform the upload
	_, roots, err := uploader.SplitableUpload(ctx, file, fragmentSize, opt)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	if len(roots) == 0 {
		return "", fmt.Errorf("no roots returned from upload")
	}

	return roots[0].String(), nil
}

// DownloadFile fetches a file from 0G Storage by its Root Hash.
func (c *StorageClient) DownloadFile(ctx context.Context, rootHash, outPath string) error {
	// The indexer client handles locating the blob and downloading it
	if err := c.indexerClient.Download(ctx, rootHash, outPath, true); err != nil {
		return fmt.Errorf("failed to download root %s: %w", rootHash, err)
	}
	return nil
}
