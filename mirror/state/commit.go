package state

import (
	"crypto/ed25519"
	"time"

	"github.com/rs/zerolog/log"
	"github.io/hashgraph/stable-coin/data"
	"github.io/hashgraph/stable-coin/domain"
)

var commitInterval = "1s"

func init() {
	commitIntervalDur, err := time.ParseDuration(commitInterval)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			time.Sleep(commitIntervalDur)
			commit()
		}
	}()
}

func commit() {
	start := time.Now()
	numOperations := 0
	numBalances := 0
	numUsers := 0
	numFreezes := 0
	numKeyUpdates := 0

	if len(pendingNewUser) > 0 {
		pendingNewUserLock.Lock()

		// there are pending operations that should be committed
		users := pendingNewUser
		numUsers = len(users)

		// erase current maps
		pendingNewUser = []string{}
		pendingNewUserLock.Unlock()

		// insert the new user records
		err := data.InsertNewAddresses(users, &User)
		if err != nil {
			panic(err)
		}
	}

	if len(pendingOperations) > 0 {
		// there are pending operations that should be committed
		pendingOperationsLock.Lock()
		operations := pendingOperations
		numOperations = len(operations)

		// erase current maps
		pendingOperations = []domain.Operation{}
		pendingOperationsForUser = map[string][]domain.Operation{}
		pendingOperationsLock.Unlock()

		// iterate and insert all the new operations
		err := data.InsertOperations(operations)
		if err != nil {
			panic(err)
		}
	}

	if len(pendingBalances) > 0 {
		pendingBalancesLock.Lock()

		// there are pending operations that should be committed
		balances := pendingBalances
		numBalances = len(balances)

		// erase current maps
		pendingBalances = map[string]uint64{}
		pendingBalancesLock.Unlock()

		// update the balance records
		err := data.UpdateUserBalances(balances)
		if err != nil {
			panic(err)
		}
	}

	if len(pendingFreezes) > 0 {
		// there are pending freezes that should be committed
		pendingFreezesLock.Lock()
		freezes := pendingFreezes
		numFreezes = len(freezes)

		// erase current maps
		pendingFreezes = map[string]bool{}
		pendingFreezesLock.Unlock()

		// update the balance records
		err := data.UpdateUserFrozenStatus(freezes)
		if err != nil {
			panic(err)
		}
	}

	if len(pendingKeyUpdates) > 0 {
		// there are pending key updates that should be committed
		pendingKeyUpdatesLock.Lock()
		keyUpdates := pendingKeyUpdates
		numKeyUpdates = len(keyUpdates)

		// erase current maps
		pendingKeyUpdates = map[string]ed25519.PublicKey{}
		pendingKeyUpdatesLock.Unlock()

		// update the address records
		err := data.UpdateKeys(keyUpdates)
		if err != nil {
			panic(err)
		}
	}

	if numOperations > 0 || numBalances > 0 || numFreezes > 0 || numKeyUpdates > 0{
		log.Info().
			Dur("elapsed", time.Since(start)).
			Int("operations", numOperations).
			Int("users", numUsers).
			Int("balances", numBalances).
			Int("freezes", numFreezes).
			Int("key updates", numKeyUpdates).
			Msg("Commit")
	}
}
