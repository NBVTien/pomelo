//go:build darwin

package commands

/*
#include <libproc.h>
#include <sys/resource.h>

static unsigned long long pom_phys_footprint(int pid) {
    struct rusage_info_v2 ri;
    if (proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&ri) != 0) return 0;
    return ri.ri_phys_footprint;
}
*/
import "C"

func physFootprintBytes(pid int) uint64 { return uint64(C.pom_phys_footprint(C.int(pid))) }
