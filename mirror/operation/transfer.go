package operation

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.io/hashgraph/stable-coin/domain"
	"github.io/hashgraph/stable-coin/mirror/api/notification"
	"github.io/hashgraph/stable-coin/mirror/state"
	"github.io/hashgraph/stable-coin/pb"
)

func Transfer(senderAddress []byte, payload *pb.Transfer) (domain.Operation, error) {
	// FIXME: proto says address but we are receiving a username
	log.Trace().
		Str("to", payload.ToAddress).
		Uint64("quantity", payload.Quantity).
		Msg("Transfer")

	var senderUserNameI interface{}
	var exists bool
	senderAddressHex := hex.EncodeToString(senderAddress)

	if senderUserNameI, exists = state.Address.Load(senderAddressHex); !exists {
		return domain.Operation{
			Operation:     domain.OpTransfer,
			Status:        domain.OpStatusFailed,
			StatusMessage: fmt.Sprintf("address `%s` does not exist", senderAddressHex),
			FromAddress:   &senderAddress,
		}, nil
	}

	senderUserName := senderUserNameI.(string)

	if _, exists = state.Balance.Load(payload.ToAddress); !exists {
		statusMessage := fmt.Sprintf("user `%s` does not exist", payload.ToAddress)
		notification.SendNotification(senderUserName, true, statusMessage)
		return domain.Operation{
			Operation:     domain.OpTransfer,
			Status:        domain.OpStatusFailed,
			StatusMessage: statusMessage,
			FromAddress:   &senderAddress,
		}, nil
	}

	toAddressI, _ := state.User.Load(payload.ToAddress)
	toAddress := []byte(toAddressI.(ed25519.PublicKey))

	if frozenUserI, exists := state.Frozen.Load(senderUserName); exists {
		if frozenUserI.(bool) == true {
			statusMessage := fmt.Sprintf("paying user `%s` is frozen", senderUserName)
			notification.SendNotification(senderUserName, true, statusMessage)
			return domain.Operation{
				Operation:     domain.OpTransfer,
				Status:        domain.OpStatusFailed,
				StatusMessage: statusMessage,
				FromAddress:   &senderAddress,
				ToAddress:     &toAddress,
			}, nil
		}
	}

	if frozenUserI, exists := state.Frozen.Load(payload.ToAddress); exists {
		if frozenUserI.(bool) == true {
			statusMessage := fmt.Sprintf("receiving user `%s` is frozen", payload.ToAddress)
			notification.SendNotification(senderUserName, true, statusMessage)
			return domain.Operation{
				Operation:     domain.OpTransfer,
				Status:        domain.OpStatusFailed,
				StatusMessage: statusMessage,
				FromAddress:   &senderAddress,
				ToAddress:     &toAddress,
			}, nil
		}
	}

	senderBalanceI, _ := state.Balance.Load(senderUserName)
	senderBalance := senderBalanceI.(uint64)

	if senderBalance < payload.Quantity {
		statusMessage := fmt.Sprintf("user `%s` has an insufficient balance", senderUserName)
		notification.SendNotification(senderUserName, true, statusMessage)
		return domain.Operation{
			Operation:     domain.OpTransfer,
			Status:        domain.OpStatusFailed,
			StatusMessage: statusMessage,
			FromAddress:   &senderAddress,
			ToAddress:     &toAddress,
		}, nil
	}

	state.UpdateBalance(payload.ToAddress, func(balance uint64) uint64 {
		return balance + payload.Quantity
	})

	state.UpdateBalance(senderUserName, func(balance uint64) uint64 {
		return balance - payload.Quantity
	})

	statusMessage := fmt.Sprintf("sent %d to `%s`", payload.Quantity, payload.ToAddress)
	notification.SendNotification(senderUserName, false, statusMessage)

	statusMessage = fmt.Sprintf("received %d from `%s`", payload.Quantity, senderUserName)
	notification.SendNotification(payload.ToAddress, false, statusMessage)

	return domain.Operation{
		Operation:   domain.OpTransfer,
		Status:      domain.OpStatusComplete,
		FromAddress: &senderAddress,
		ToAddress:   &toAddress,
		Amount:      int64(payload.Quantity),
	}, nil
}
