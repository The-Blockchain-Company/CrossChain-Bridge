package routerswap

import (
	"github.com/anyswap/CrossChain-Bridge/mongodb"
	"github.com/anyswap/CrossChain-Bridge/params"
	"github.com/anyswap/CrossChain-Bridge/tokens"
	"github.com/anyswap/CrossChain-Bridge/tokens/router"
)

// StartVerifyJob verify job
func StartVerifyJob() {
	logWorker("verify", "start router swap verify job")
	for {
		septime := getSepTimeInFind(maxVerifyLifetime)
		res, err := mongodb.FindRouterSwapsWithStatus(mongodb.TxNotStable, septime)
		if err != nil {
			logWorkerError("verify", "find router swap error", err)
		}
		if len(res) > 0 {
			logWorker("verify", "find router swap to verify", "count", len(res))
		}
		for _, swap := range res {
			err = processRouterSwapVerify(swap)
			switch err {
			case nil, tokens.ErrTxNotStable, tokens.ErrTxNotFound:
			default:
				logWorkerError("verify", "verify router swap error", err, "chainid", swap.FromChainID, "txid", swap.TxID, "logIndex", swap.LogIndex)
			}
		}
		restInJob(restIntervalInVerifyJob)
	}
}

func processRouterSwapVerify(swap *mongodb.MgoSwap) (err error) {
	fromChainID := swap.FromChainID
	txid := swap.TxID
	logIndex := swap.LogIndex

	if params.IsSwapInBlacklist(fromChainID, swap.ToChainID, swap.TokenID) {
		err = tokens.ErrSwapInBlacklist
		_ = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.SwapInBlacklist, now(), err.Error())
		return err
	}

	bridge := router.GetBridgeByChainID(fromChainID)
	swapInfo, err := bridge.VerifyRouterSwapTx(txid, logIndex, false)

	if swapInfo.Height != 0 && swapInfo.Height < bridge.ChainConfig.InitialHeight {
		err = tokens.ErrTxBeforeInitialHeight
	}

	var dbErr error

	switch err {
	case nil:
		if swapInfo.Value.Cmp(bridge.GetBigValueThreshold(swapInfo.Token)) > 0 {
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithBigValue, now(), "")
		} else {
			dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxNotSwapped, now(), "")
			if dbErr == nil {
				dbErr = addInitialSwapResult(swapInfo, mongodb.MatchTxEmpty)
			}
		}
	case tokens.ErrTxNotStable, tokens.ErrTxNotFound:
		break
	case tokens.ErrTxWithWrongValue:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongValue, now(), err.Error())
	case tokens.ErrTxWithWrongPath:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxWithWrongPath, now(), err.Error())
	case tokens.ErrMissTokenConfig:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.MissTokenConfig, now(), err.Error())
	case tokens.ErrNoUnderlyingToken:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.NoUnderlyingToken, now(), err.Error())
	default:
		dbErr = mongodb.UpdateRouterSwapStatus(fromChainID, txid, logIndex, mongodb.TxVerifyFailed, now(), err.Error())
	}

	if dbErr != nil {
		logWorker("verify", "verify router swap db error", "dbErr", dbErr, "verifyErr", err, "fromChainID", fromChainID, "txid", txid, "logIndex", logIndex)
	}

	return err
}