package routerswap

import (
	"github.com/anyswap/CrossChain-Bridge/mongodb"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/types"
)

// StartStableJob stable job
func StartStableJob() {
	logWorker("stable", "start router swap stable job")
	for {
		res, err := findRouterSwapResultsToStable()
		if err != nil {
			logWorkerError("stable", "find router swap results error", err)
		}
		if len(res) > 0 {
			logWorker("stable", "find router swap results to stable", "count", len(res))
		}
		for _, swap := range res {
			err = processRouterSwapStable(swap)
			if err != nil {
				logWorkerError("stable", "process router swap stable error", err, "chainID", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		restInJob(restIntervalInStableJob)
	}
}

func findRouterSwapResultsToStable() ([]*mongodb.MgoSwapResult, error) {
	status := mongodb.MatchTxNotStable
	septime := getSepTimeInFind(maxStableLifetime)
	return mongodb.FindRouterSwapResultsWithStatus(status, septime)
}

func isTxOnChain(txStatus *tokens.TxStatus) bool {
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return false
	}
	return txStatus.Receipt != nil
}

func getSwapTxStatus(resBridge tokens.CrossChainBridge, swap *mongodb.MgoSwapResult) *tokens.TxStatus {
	txStatus := resBridge.GetTransactionStatus(swap.SwapTx)
	if isTxOnChain(txStatus) {
		return txStatus
	}
	for _, oldSwapTx := range swap.OldSwapTxs {
		if swap.SwapTx == oldSwapTx {
			continue
		}
		txStatus2 := resBridge.GetTransactionStatus(oldSwapTx)
		if isTxOnChain(txStatus2) {
			swap.SwapTx = oldSwapTx
			return txStatus2
		}
	}
	return txStatus
}

func processRouterSwapStable(swap *mongodb.MgoSwapResult) (err error) {
	oldSwapTx := swap.SwapTx
	resBridge := tokens.GetCrossChainBridgeByChainID(swap.ToChainID)
	txStatus := getSwapTxStatus(resBridge, swap)
	if txStatus == nil || txStatus.BlockHeight == 0 {
		return nil
	}

	if swap.SwapHeight != 0 {
		if txStatus.Confirmations < *resBridge.GetChainConfig().Confirmations {
			return nil
		}
		if swap.SwapTx != oldSwapTx {
			_ = updateSwapTx(swap.FromChainID, swap.TxID, swap.LogIndex, swap.SwapTx)
		}
		if txStatus.Receipt != nil {
			receipt, ok := txStatus.Receipt.(*types.RPCTxReceipt)
			txFailed := !ok || receipt == nil || *receipt.Status != 1
			token := resBridge.GetTokenConfig(swap.PairID)
			if !txFailed && token != nil && token.ContractAddress != "" && len(receipt.Logs) == 0 {
				txFailed = true
			}
			if txFailed {
				return markSwapResultFailed(swap.FromChainID, swap.TxID, swap.LogIndex)
			}
		}
		return markSwapResultStable(swap.FromChainID, swap.TxID, swap.LogIndex)
	}

	matchTx := &MatchTx{
		SwapHeight: txStatus.BlockHeight,
		SwapTime:   txStatus.BlockTime,
	}
	if swap.SwapTx != oldSwapTx {
		matchTx.SwapTx = swap.SwapTx
	}
	return updateRouterSwapResult(swap.FromChainID, swap.TxID, swap.LogIndex, matchTx)
}
