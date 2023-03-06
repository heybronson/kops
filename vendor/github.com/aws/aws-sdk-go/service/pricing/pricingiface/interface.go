// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

// Package pricingiface provides an interface to enable mocking the AWS Price List Service service client
// for testing your code.
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters.
package pricingiface

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/pricing"
)

// PricingAPI provides an interface to enable mocking the
// pricing.Pricing service client's API operation,
// paginators, and waiters. This make unit testing your code that calls out
// to the SDK's service client's calls easier.
//
// The best way to use this interface is so the SDK's service client's calls
// can be stubbed out for unit testing your code with the SDK without needing
// to inject custom request handlers into the SDK's request pipeline.
//
//	// myFunc uses an SDK service client to make a request to
//	// AWS Price List Service.
//	func myFunc(svc pricingiface.PricingAPI) bool {
//	    // Make svc.DescribeServices request
//	}
//
//	func main() {
//	    sess := session.New()
//	    svc := pricing.New(sess)
//
//	    myFunc(svc)
//	}
//
// In your _test.go file:
//
//	// Define a mock struct to be used in your unit tests of myFunc.
//	type mockPricingClient struct {
//	    pricingiface.PricingAPI
//	}
//	func (m *mockPricingClient) DescribeServices(input *pricing.DescribeServicesInput) (*pricing.DescribeServicesOutput, error) {
//	    // mock response/functionality
//	}
//
//	func TestMyFunc(t *testing.T) {
//	    // Setup Test
//	    mockSvc := &mockPricingClient{}
//
//	    myfunc(mockSvc)
//
//	    // Verify myFunc's functionality
//	}
//
// It is important to note that this interface will have breaking changes
// when the service model is updated and adds new API operations, paginators,
// and waiters. Its suggested to use the pattern above for testing, or using
// tooling to generate mocks to satisfy the interfaces.
type PricingAPI interface {
	DescribeServices(*pricing.DescribeServicesInput) (*pricing.DescribeServicesOutput, error)
	DescribeServicesWithContext(aws.Context, *pricing.DescribeServicesInput, ...request.Option) (*pricing.DescribeServicesOutput, error)
	DescribeServicesRequest(*pricing.DescribeServicesInput) (*request.Request, *pricing.DescribeServicesOutput)

	DescribeServicesPages(*pricing.DescribeServicesInput, func(*pricing.DescribeServicesOutput, bool) bool) error
	DescribeServicesPagesWithContext(aws.Context, *pricing.DescribeServicesInput, func(*pricing.DescribeServicesOutput, bool) bool, ...request.Option) error

	GetAttributeValues(*pricing.GetAttributeValuesInput) (*pricing.GetAttributeValuesOutput, error)
	GetAttributeValuesWithContext(aws.Context, *pricing.GetAttributeValuesInput, ...request.Option) (*pricing.GetAttributeValuesOutput, error)
	GetAttributeValuesRequest(*pricing.GetAttributeValuesInput) (*request.Request, *pricing.GetAttributeValuesOutput)

	GetAttributeValuesPages(*pricing.GetAttributeValuesInput, func(*pricing.GetAttributeValuesOutput, bool) bool) error
	GetAttributeValuesPagesWithContext(aws.Context, *pricing.GetAttributeValuesInput, func(*pricing.GetAttributeValuesOutput, bool) bool, ...request.Option) error

	GetPriceListFileUrl(*pricing.GetPriceListFileUrlInput) (*pricing.GetPriceListFileUrlOutput, error)
	GetPriceListFileUrlWithContext(aws.Context, *pricing.GetPriceListFileUrlInput, ...request.Option) (*pricing.GetPriceListFileUrlOutput, error)
	GetPriceListFileUrlRequest(*pricing.GetPriceListFileUrlInput) (*request.Request, *pricing.GetPriceListFileUrlOutput)

	GetProducts(*pricing.GetProductsInput) (*pricing.GetProductsOutput, error)
	GetProductsWithContext(aws.Context, *pricing.GetProductsInput, ...request.Option) (*pricing.GetProductsOutput, error)
	GetProductsRequest(*pricing.GetProductsInput) (*request.Request, *pricing.GetProductsOutput)

	GetProductsPages(*pricing.GetProductsInput, func(*pricing.GetProductsOutput, bool) bool) error
	GetProductsPagesWithContext(aws.Context, *pricing.GetProductsInput, func(*pricing.GetProductsOutput, bool) bool, ...request.Option) error

	ListPriceLists(*pricing.ListPriceListsInput) (*pricing.ListPriceListsOutput, error)
	ListPriceListsWithContext(aws.Context, *pricing.ListPriceListsInput, ...request.Option) (*pricing.ListPriceListsOutput, error)
	ListPriceListsRequest(*pricing.ListPriceListsInput) (*request.Request, *pricing.ListPriceListsOutput)

	ListPriceListsPages(*pricing.ListPriceListsInput, func(*pricing.ListPriceListsOutput, bool) bool) error
	ListPriceListsPagesWithContext(aws.Context, *pricing.ListPriceListsInput, func(*pricing.ListPriceListsOutput, bool) bool, ...request.Option) error
}

var _ PricingAPI = (*pricing.Pricing)(nil)
