package client

import (
	"context"

	"github.com/sirupsen/logrus"

	commonClient "github.com/AccelByte/extend-challenge-common/pkg/client"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// NoOpRewardClient is a no-op implementation of RewardClient for M1.
// It logs reward grants instead of making actual AGS Platform Service calls.
// This allows the service to run without AGS integration for local development and testing.
//
// In Phase 7, this will be replaced with a real AGS SDK client that calls:
// - AGS Platform Service for item entitlements
// - AGS E-Commerce Service for wallet credits
//
// Usage:
//
//	rewardClient := client.NewNoOpRewardClient(logger)
//	err := rewardClient.GrantReward(ctx, namespace, userID, reward)
type NoOpRewardClient struct {
	logger *logrus.Logger
}

// NewNoOpRewardClient creates a new no-op reward client
func NewNoOpRewardClient(logger *logrus.Logger) commonClient.RewardClient {
	return &NoOpRewardClient{logger: logger}
}

// GrantItemReward logs the item reward grant instead of calling AGS
func (c *NoOpRewardClient) GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error {
	c.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"user_id":   userID,
		"item_id":   itemID,
		"quantity":  quantity,
	}).Info("[NO-OP] Would grant item reward (AGS integration in Phase 7)")
	return nil
}

// GrantWalletReward logs the wallet reward grant instead of calling AGS
func (c *NoOpRewardClient) GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error {
	c.logger.WithFields(logrus.Fields{
		"namespace":     namespace,
		"user_id":       userID,
		"currency_code": currencyCode,
		"amount":        amount,
	}).Info("[NO-OP] Would grant wallet reward (AGS integration in Phase 7)")
	return nil
}

// GrantReward dispatches to the appropriate grant method based on reward type
func (c *NoOpRewardClient) GrantReward(ctx context.Context, namespace, userID string, reward commonDomain.Reward) error {
	switch reward.Type {
	case "ITEM":
		return c.GrantItemReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	case "WALLET":
		return c.GrantWalletReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	default:
		c.logger.WithFields(logrus.Fields{
			"namespace":   namespace,
			"user_id":     userID,
			"reward_type": reward.Type,
		}).Warn("[NO-OP] Unknown reward type")
		return nil
	}
}
