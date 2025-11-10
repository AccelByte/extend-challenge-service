package client

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

func TestNewNoOpRewardClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress logs in tests

	client := NewNoOpRewardClient(logger)

	assert.NotNil(t, client)
	noopClient, ok := client.(*NoOpRewardClient)
	assert.True(t, ok, "Client should be *NoOpRewardClient")
	assert.NotNil(t, noopClient.logger)
}

func TestNoOpRewardClient_GrantItemReward(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress logs in tests

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	err := client.GrantItemReward(ctx, "test-namespace", "user123", "sword", 1)

	// NoOp should always succeed (return nil)
	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantItemReward_MultipleQuantity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	err := client.GrantItemReward(ctx, "test-namespace", "user456", "potion", 10)

	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantWalletReward(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	err := client.GrantWalletReward(ctx, "test-namespace", "user123", "GOLD", 100)

	// NoOp should always succeed (return nil)
	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantWalletReward_LargeAmount(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	err := client.GrantWalletReward(ctx, "test-namespace", "user789", "GEMS", 1000)

	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantReward_ItemType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "ITEM",
		RewardID: "sword",
		Quantity: 1,
	}

	err := client.GrantReward(ctx, "test-namespace", "user123", reward)

	// NoOp should always succeed
	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantReward_WalletType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "WALLET",
		RewardID: "GOLD",
		Quantity: 500,
	}

	err := client.GrantReward(ctx, "test-namespace", "user456", reward)

	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantReward_UnknownType(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "UNKNOWN_TYPE",
		RewardID: "something",
		Quantity: 10,
	}

	err := client.GrantReward(ctx, "test-namespace", "user789", reward)

	// NoOp should handle unknown types gracefully (log warning and return nil)
	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantReward_EmptyRewardID(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "ITEM",
		RewardID: "",
		Quantity: 1,
	}

	err := client.GrantReward(ctx, "test-namespace", "user123", reward)

	// NoOp should handle empty reward ID gracefully
	assert.NoError(t, err)
}

func TestNoOpRewardClient_GrantReward_ZeroQuantity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &NoOpRewardClient{logger: logger}

	ctx := context.Background()
	reward := commonDomain.Reward{
		Type:     "ITEM",
		RewardID: "sword",
		Quantity: 0,
	}

	err := client.GrantReward(ctx, "test-namespace", "user123", reward)

	// NoOp should handle zero quantity gracefully
	assert.NoError(t, err)
}
