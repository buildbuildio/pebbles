package common

import (
	"strings"
)

const (
	IDFieldName = "id"

	NodeFieldName     = "node"
	NodeInterfaceName = "Node"

	QueryObjectName        = "Query"
	MutationObjectName     = "Mutation"
	SubscriptionObjectName = "Subscription"

	TypenameFieldName = "__typename"

	InternalServiceName = "%#!"
)

// isBuiltInName returns true, if it's a default GQL schema field name, f.e. __typename
func IsBuiltinName(s string) bool {
	return strings.HasPrefix(s, "__")
}

// isNodeInterfaceName returns true if name is Node
func IsNodeInterfaceName(s string) bool {
	return s == NodeInterfaceName
}

// isQueryObjectName returns true if name is Query
func IsQueryObjectName(s string) bool {
	return s == QueryObjectName
}

// isMutationObjectName returns true if name is Mutation
func IsMutationObjectName(s string) bool {
	return s == MutationObjectName
}

// isSubscriptionObjectName returns true if name is Subscription
func IsSubscriptionObjectName(s string) bool {
	return s == SubscriptionObjectName
}

// isRootObjectName returns true if name is Query, Mutation or Subscription
func IsRootObjectName(s string) bool {
	return IsQueryObjectName(s) || IsMutationObjectName(s) || IsSubscriptionObjectName(s)
}
