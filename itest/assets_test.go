package itest

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightninglabs/taro/tarorpc"
	"github.com/stretchr/testify/require"
)

var (
	zeroHash chainhash.Hash

	simpleAssets = []*tarorpc.MintAssetRequest{
		{
			AssetType: tarorpc.AssetType_NORMAL,
			Name:      "itestbuxx",
			MetaData:  []byte("some metadata for the itest assets"),
			Amount:    5000,
		},
		{
			AssetType: tarorpc.AssetType_COLLECTIBLE,
			Name:      "itestbuxx-collectible",
			MetaData:  []byte("some metadata for the itest assets"),
			Amount:    1,
		},
	}
	issuableAssets = []*tarorpc.MintAssetRequest{
		{
			AssetType:      tarorpc.AssetType_NORMAL,
			Name:           "itestbuxx-money-printer-brrr",
			MetaData:       []byte("some metadata"),
			Amount:         5000,
			EnableEmission: true,
		},
		{
			AssetType:      tarorpc.AssetType_COLLECTIBLE,
			Name:           "itestbuxx-collectible-brrr",
			MetaData:       []byte("some metadata"),
			Amount:         1,
			EnableEmission: true,
		},
	}
)

func mintAssets(t *harnessTest) {
	rpcSimpleAssets := mintAssetsConfirmBatch(t, t.tarod, simpleAssets)
	rpcIssuableAssets := mintAssetsConfirmBatch(t, t.tarod, issuableAssets)

	// Now that all our assets have been issued, we'll use the balance
	// calls to ensure that we're able to retrieve the proper balance for
	// them all.
	assertAssetBalances(t, rpcSimpleAssets, rpcIssuableAssets)

	// Make sure the proof files for the freshly minted assets can be
	// retrieved and are fully valid.
	var allAssets []*tarorpc.Asset
	allAssets = append(allAssets, rpcSimpleAssets...)
	allAssets = append(allAssets, rpcIssuableAssets...)
	for _, mintedAsset := range allAssets {
		assertAssetProofs(t.t, t.tarod, mintedAsset)
	}

	// Let's now create a new node and import all assets into that new node.
	charlie := t.lndHarness.NewNode(t.t, "charlie", lndDefaultArgs)
	secondTarod := setupTarodHarness(
		t.t, t, t.lndHarness.BackendCfg, charlie, t.universeServer,
	)
	defer shutdownAndAssert(t, charlie, secondTarod)

	transferAssetProofs(t, t.tarod, secondTarod, allAssets)
}

// mintAssetsConfirmBatch mints all given assets in the same batch, confirms the
// batch and verifies all asset proofs of the minted assets.
func mintAssetsConfirmBatch(t *harnessTest, tarod *tarodHarness,
	assetRequests []*tarorpc.MintAssetRequest) []*tarorpc.Asset {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultWaitTimeout)
	defer cancel()

	// Mint all the assets in the same batch.
	for idx, assetRequest := range assetRequests {
		// Trigger a new batch with the last asset. The name SkipBatch
		// is a bit misleading in this context. It basically means:
		// Don't allow adding more assets to the batch, ship it now.
		if idx == len(assetRequests)-1 {
			assetRequest.SkipBatch = true
		}

		assetResp, err := tarod.MintAsset(ctxt, assetRequest)
		require.NoError(t.t, err)
		require.NotEmpty(t.t, assetResp.BatchKey)
	}

	hashes, err := waitForNTxsInMempool(
		t.lndHarness.Miner.Client, 1, defaultWaitTimeout,
	)
	require.NoError(t.t, err)

	// Make sure the assets were all minted within the same anchor but don't
	// yet have a block hash associated with them.
	for _, assetRequest := range assetRequests {
		assertAssetState(
			t, tarod, assetRequest.Name, assetRequest.MetaData,
			assetAmountCheck(assetRequest.Amount),
			assetTypeCheck(assetRequest.AssetType),
			assetAnchorCheck(*hashes[0], zeroHash),
		)
	}

	// Mine a block to confirm the assets.
	block := mineBlocks(t, t.lndHarness, 1, 1)[0]
	blockHash := block.BlockHash()

	// The rest of the anchor information should now be populated as well.
	// We also check that the anchor outpoint of all assets is the same,
	// since they were all minted in the same batch.
	var (
		firstOutpoint string
		assetList     []*tarorpc.Asset
	)
	for _, assetRequest := range assetRequests {
		mintedAsset := assertAssetState(
			t, tarod, assetRequest.Name, assetRequest.MetaData,
			assetAnchorCheck(*hashes[0], blockHash),
			func(a *tarorpc.Asset) error {
				anchor := a.ChainAnchor

				if anchor.AnchorOutpoint == "" {
					return fmt.Errorf("missing anchor " +
						"outpoint")
				}

				if firstOutpoint == "" {
					firstOutpoint = anchor.AnchorOutpoint

					return nil
				}

				if anchor.AnchorOutpoint != firstOutpoint {
					return fmt.Errorf("unexpected anchor "+
						"outpoint, got %v wanted %v",
						anchor.AnchorOutpoint,
						firstOutpoint)
				}

				return nil
			},
		)

		assetList = append(assetList, mintedAsset)
	}

	return assetList
}

// transferAssetProofs locates and exports the proof files for all given assets
// from the source node and imports them into the destination node.
func transferAssetProofs(t *harnessTest, src, dst *tarodHarness,
	assets []*tarorpc.Asset) {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultWaitTimeout)
	defer cancel()

	// TODO(roasbeef): modify import call, can't work as is
	//  * proof file only contains the tweaked script key
	//  * from that we don't know the internal key
	//  * we can import the proof but it's useless as is, but lets this
	//  itest work

	for _, existingAsset := range assets {
		gen := existingAsset.AssetGenesis
		proofFile := assertAssetProofs(t.t, src, existingAsset)
		_, err := dst.ImportProof(ctxt, &tarorpc.ImportProofRequest{
			ProofFile:    proofFile,
			GenesisPoint: gen.GenesisPoint,
		})
		require.NoError(t.t, err)

		anchorTxHash, err := chainhash.NewHash(
			existingAsset.ChainAnchor.AnchorTxid,
		)
		require.NoError(t.t, err)
		anchorBlockHash, err := chainhash.NewHash(
			existingAsset.ChainAnchor.AnchorBlockHash,
		)
		require.NoError(t.t, err)

		assertAssetState(
			t, dst, gen.Name, gen.Meta,
			assetAmountCheck(existingAsset.Amount),
			assetTypeCheck(existingAsset.AssetType),
			assetAnchorCheck(*anchorTxHash, *anchorBlockHash),
		)
	}
}

func assertAssetBalances(t *harnessTest,
	simpleAssets, issuableAssets []*tarorpc.Asset) {

	t.t.Helper()

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultWaitTimeout)
	defer cancel()

	// First, we'll ensure that we're able to get the balances of all the
	// assets grouped by their asset IDs.
	balanceReq := &tarorpc.ListBalancesRequest_AssetId{
		AssetId: true,
	}
	assetIDBalances, err := t.tarod.ListBalances(
		ctxt, &tarorpc.ListBalancesRequest{
			GroupBy: balanceReq,
		},
	)
	require.NoError(t.t, err)

	var allAssets []*tarorpc.Asset
	allAssets = append(allAssets, simpleAssets...)
	allAssets = append(allAssets, issuableAssets...)

	require.Equal(t.t, len(allAssets), len(assetIDBalances.AssetBalances))

	for _, balance := range assetIDBalances.AssetBalances {
		for _, rpcAsset := range allAssets {
			if balance.AssetGenesis.Name == rpcAsset.AssetGenesis.Name {
				require.Equal(
					t.t, balance.Balance, rpcAsset.Amount,
				)
				require.Equal(
					t.t,
					balance.AssetGenesis.GenesisBootstrapInfo,
					rpcAsset.AssetGenesis.GenesisBootstrapInfo,
				)
			}
		}
	}

	// We'll also ensure that we're able to get the balance by key family
	// for all the assets that have one specified.
	famBalanceReq := &tarorpc.ListBalancesRequest_FamKey{
		FamKey: true,
	}
	assetFamBalances, err := t.tarod.ListBalances(
		ctxt, &tarorpc.ListBalancesRequest{
			GroupBy: famBalanceReq,
		},
	)
	require.NoError(t.t, err)

	require.Equal(
		t.t, len(issuableAssets),
		len(assetFamBalances.AssetFamilyBalances),
	)

	for _, balance := range assetFamBalances.AssetBalances {
		for _, rpcAsset := range issuableAssets {
			if balance.AssetGenesis.Name == rpcAsset.AssetGenesis.Name {
				require.Equal(
					t.t, balance.Balance, rpcAsset.Amount,
				)
				require.Equal(
					t.t,
					balance.AssetGenesis.GenesisBootstrapInfo,
					rpcAsset.AssetGenesis.GenesisBootstrapInfo,
				)
			}
		}
	}
}
