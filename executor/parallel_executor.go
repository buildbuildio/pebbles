// Originally nautilus ParallelExecutor runs all queries async, which leads to insufficient usage of queryer cache,
// low readability of code and many race conditions, which can potentially lead to data loss.
//
// Proposed solution works differently: for each graph node it computes depth inside graph, accumulates all nodes on same depth.
// After that it walks sequently from depth to depth, executing all requests on same depth in parallel.
//
// Scheme as example:
// {
//     users {
//         id
//         name
//         books {
//             id
//             name
//         }
//         permissisons {
//             id
//         }
//     }
// }

// Service A -> users
// Service B -> books
// Service C -> permissions

// depth 0               -> depth 1                  -> depth 2
// {system query}        -> Plan 1 (users {id name}) -> Plan 2 (users {books {id name}})
//                                                   -> Plan 3 (users {permissions {id}})

package executor

// ParallelExecutor executes the given query plan by starting at the root of the plan and
// walking down the path stitching the results together
type ParallelExecutor func(*ExecutionContext) (map[string]interface{}, error)

// Execute returns the result of the query plan
// for more information about usage, take a look at tests.
func (executor ParallelExecutor) Execute(ctx *ExecutionContext) (map[string]interface{}, error) {
	manager := NewDepthExecutorManager(ctx)
	res, err := manager.Execute()
	if err != nil {
		return nil, err
	}
	return res, nil
}
