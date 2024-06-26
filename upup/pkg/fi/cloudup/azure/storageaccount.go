/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// StorageAccountsClient is a client for managing Network Interfaces.
type StorageAccountsClient interface {
	List(ctx context.Context) ([]*armstorage.Account, error)
}

type storageAccountsClientImpl struct {
	c *armstorage.AccountsClient
}

var _ StorageAccountsClient = &storageAccountsClientImpl{}

func (c *storageAccountsClientImpl) List(ctx context.Context) ([]*armstorage.Account, error) {
	var l []*armstorage.Account
	pager := c.c.NewListPager(nil)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing storage accounts: %w", err)
		}
		l = append(l, resp.Value...)
	}
	return l, nil
}

func newStorageAccountsClientImpl(subscriptionID string, cred *azidentity.DefaultAzureCredential) (*storageAccountsClientImpl, error) {
	c, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating storage accounts client: %w", err)
	}
	return &storageAccountsClientImpl{
		c: c,
	}, nil
}
