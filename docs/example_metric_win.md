```text
# HELP node_cpu_seconds_total Seconds the CPUs spent in each mode.
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="cpu0",mode="user"} 452.12
node_cpu_seconds_total{cpu="cpu0",mode="system"} 120.50
node_cpu_seconds_total{cpu="cpu0",mode="idle"} 8012.30
node_cpu_seconds_total{cpu="cpu0",mode="irq"} 5.40
node_cpu_seconds_total{cpu="cpu0",mode="softirq"} 12.10
# HELP node_cpu_count Number of logical processors.
# TYPE node_cpu_count gauge
node_cpu_count 4
# HELP node_context_switches_per_sec Context switches per second.
# TYPE node_context_switches_per_sec gauge
node_context_switches_per_sec 6103.4
# HELP node_procs_running Total number of processes.
# TYPE node_procs_running gauge
node_procs_running 145

# HELP node_memory_MemTotal_bytes Memory information field MemTotal.
# TYPE node_memory_MemTotal_bytes gauge
node_memory_MemTotal_bytes 1.7061732352e+10
# HELP node_memory_MemFree_bytes Memory information field MemFree.
# TYPE node_memory_MemFree_bytes gauge
node_memory_MemFree_bytes 8.589934592e+09

# HELP node_disk_reads_completed_total Total reads completed.
# TYPE node_disk_reads_completed_total counter
node_disk_reads_completed_total{device="C:"} 154231
# HELP node_disk_read_bytes_total Total bytes read.
# TYPE node_disk_read_bytes_total counter
node_disk_read_bytes_total{device="C:"} 5.36870912e+09

# HELP node_network_receive_bytes_total Network bytes received.
# TYPE node_network_receive_bytes_total counter
node_network_receive_bytes_total{device="Ethernet"} 4.294967296e+09

# HELP node_load1 1 minute load average.
# TYPE node_load1 gauge
node_load1 0.45

# HELP node_filesystem_size_bytes Filesystem size in bytes.
# TYPE node_filesystem_size_bytes gauge
node_filesystem_size_bytes{device="C:",fstype="NTFS"} 5.12110190592e+11
# HELP node_filesystem_free_bytes Filesystem free space.
# TYPE node_filesystem_free_bytes gauge
node_filesystem_free_bytes{device="C:",fstype="NTFS"} 1.2884901888e+11

# HELP node_boot_time_seconds Node boot time in seconds since epoch.
# TYPE node_boot_time_seconds gauge
node_boot_time_seconds 1.77307000e+09
# HELP node_time_seconds Current system time in seconds since epoch.
# TYPE node_time_seconds gauge
node_time_seconds 1.77333572e+09
```
