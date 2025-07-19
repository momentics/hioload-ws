// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// NUMA topology detection via libnuma/hwloc and proper thread pinning.

package concurrency

/*
#cgo LDFLAGS: -lnuma -lhwloc
#include <numa.h>
#include <hwloc.h>
*/
import "C"

// PreferredCPUID returns logical CPU index for given NUMA node.
func PreferredCPUID(numaNode int) int {
    topo := C.hwloc_topology_alloc()
    C.hwloc_topology_load(topo)
    objs := C.hwloc_get_obj_by_type(topo, C.HWLOC_OBJ_NODE, C.uint(numaNode))
    if objs == nil {
        return 0
    }
    core := C.hwloc_get_obj_inside_cpuset_by_type(topo, objs.cpuset, C.HWLOC_OBJ_CORE, 0)
    return int(core.logical_index)
}

// CurrentNUMANodeID returns NUMA node of current CPU.
func CurrentNUMANodeID() int {
    if C.numa_available() < 0 {
        return 0
    }
    cpu := C.sched_getcpu()
    return int(C.numa_node_of_cpu(cpu))
}
