```text
# HELP node_cpu_seconds_total Seconds the CPUs spent in each mode.
# TYPE node_cpu_seconds_total counter
node_cpu_seconds_total{cpu="cpu0",mode="user"} 15823.47
node_cpu_seconds_total{cpu="cpu0",mode="nice"} 2.31
node_cpu_seconds_total{cpu="cpu0",mode="system"} 4891.62
node_cpu_seconds_total{cpu="cpu0",mode="idle"} 238471.05
node_cpu_seconds_total{cpu="cpu0",mode="iowait"} 312.78
node_cpu_seconds_total{cpu="cpu0",mode="irq"} 0
node_cpu_seconds_total{cpu="cpu0",mode="softirq"} 87.44
node_cpu_seconds_total{cpu="cpu0",mode="steal"} 0
node_cpu_seconds_total{cpu="cpu1",mode="user"} 14297.18
node_cpu_seconds_total{cpu="cpu1",mode="nice"} 1.89
node_cpu_seconds_total{cpu="cpu1",mode="system"} 4562.33
node_cpu_seconds_total{cpu="cpu1",mode="idle"} 239812.71
node_cpu_seconds_total{cpu="cpu1",mode="iowait"} 287.42
node_cpu_seconds_total{cpu="cpu1",mode="irq"} 0
node_cpu_seconds_total{cpu="cpu1",mode="softirq"} 64.19
node_cpu_seconds_total{cpu="cpu1",mode="steal"} 0
# HELP node_cpu_count Number of CPUs.
# TYPE node_cpu_count gauge
node_cpu_count 2
# HELP node_context_switches_total Total context switches.
# TYPE node_context_switches_total counter
node_context_switches_total 4.82937615e+08
# HELP node_forks_total Total forks.
# TYPE node_forks_total counter
node_forks_total 1.247832e+06
# HELP node_procs_running Number of processes in runnable state.
# TYPE node_procs_running gauge
node_procs_running 3
# HELP node_procs_blocked Number of processes blocked waiting for I/O.
# TYPE node_procs_blocked gauge
node_procs_blocked 0

# HELP node_memory_MemTotal_bytes Memory information field MemTotal.
# TYPE node_memory_MemTotal_bytes gauge
node_memory_MemTotal_bytes 8.242855936e+09
# HELP node_memory_MemFree_bytes Memory information field MemFree.
# TYPE node_memory_MemFree_bytes gauge
node_memory_MemFree_bytes 2.4182784e+09
# HELP node_memory_MemAvailable_bytes Memory information field MemAvailable.
# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 5.124390912e+09
# HELP node_memory_Buffers_bytes Memory information field Buffers.
# TYPE node_memory_Buffers_bytes gauge
node_memory_Buffers_bytes 2.68435456e+08
# HELP node_memory_Cached_bytes Memory information field Cached.
# TYPE node_memory_Cached_bytes gauge
node_memory_Cached_bytes 2.684354560e+09
# HELP node_memory_Active_bytes Memory information field Active.
# TYPE node_memory_Active_bytes gauge
node_memory_Active_bytes 3.489660928e+09
# HELP node_memory_Inactive_bytes Memory information field Inactive.
# TYPE node_memory_Inactive_bytes gauge
node_memory_Inactive_bytes 1.610612736e+09
# HELP node_memory_SwapTotal_bytes Memory information field SwapTotal.
# TYPE node_memory_SwapTotal_bytes gauge
node_memory_SwapTotal_bytes 2.147483648e+09
# HELP node_memory_SwapFree_bytes Memory information field SwapFree.
# TYPE node_memory_SwapFree_bytes gauge
node_memory_SwapFree_bytes 2.113929216e+09
# HELP node_memory_Dirty_bytes Memory information field Dirty.
# TYPE node_memory_Dirty_bytes gauge
node_memory_Dirty_bytes 516096
# HELP node_memory_Shmem_bytes Memory information field Shmem.
# TYPE node_memory_Shmem_bytes gauge
node_memory_Shmem_bytes 6.7108864e+07

# HELP node_disk_reads_completed_total Total reads completed.
# TYPE node_disk_reads_completed_total counter
node_disk_reads_completed_total{device="sda"} 284519
# HELP node_disk_read_bytes_total Total bytes read.
# TYPE node_disk_read_bytes_total counter
node_disk_read_bytes_total{device="sda"} 7.516192768e+09
# HELP node_disk_read_time_seconds_total Total read time.
# TYPE node_disk_read_time_seconds_total counter
node_disk_read_time_seconds_total{device="sda"} 142.876
# HELP node_disk_writes_completed_total Total writes completed.
# TYPE node_disk_writes_completed_total counter
node_disk_writes_completed_total{device="sda"} 1.024783e+06
# HELP node_disk_written_bytes_total Total bytes written.
# TYPE node_disk_written_bytes_total counter
node_disk_written_bytes_total{device="sda"} 2.1474836480e+10
# HELP node_disk_write_time_seconds_total Total write time.
# TYPE node_disk_write_time_seconds_total counter
node_disk_write_time_seconds_total{device="sda"} 587.224
# HELP node_disk_io_now I/Os currently in progress.
# TYPE node_disk_io_now gauge
node_disk_io_now{device="sda"} 0
# HELP node_disk_io_time_seconds_total Total time doing I/Os.
# TYPE node_disk_io_time_seconds_total counter
node_disk_io_time_seconds_total{device="sda"} 412.548

# HELP node_network_receive_bytes_total Network bytes received.
# TYPE node_network_receive_bytes_total counter
node_network_receive_bytes_total{device="eth0"} 1.8274538496e+10
# HELP node_network_receive_packets_total Network packets received.
# TYPE node_network_receive_packets_total counter
node_network_receive_packets_total{device="eth0"} 2.4891573e+07
# HELP node_network_receive_errs_total Network receive errors.
# TYPE node_network_receive_errs_total counter
node_network_receive_errs_total{device="eth0"} 0
# HELP node_network_receive_drop_total Network receive drops.
# TYPE node_network_receive_drop_total counter
node_network_receive_drop_total{device="eth0"} 147
# HELP node_network_transmit_bytes_total Network bytes transmitted.
# TYPE node_network_transmit_bytes_total counter
node_network_transmit_bytes_total{device="eth0"} 3.221225472e+09
# HELP node_network_transmit_packets_total Network packets transmitted.
# TYPE node_network_transmit_packets_total counter
node_network_transmit_packets_total{device="eth0"} 8.127364e+06
# HELP node_network_transmit_errs_total Network transmit errors.
# TYPE node_network_transmit_errs_total counter
node_network_transmit_errs_total{device="eth0"} 0
# HELP node_network_transmit_drop_total Network transmit drops.
# TYPE node_network_transmit_drop_total counter
node_network_transmit_drop_total{device="eth0"} 0

# HELP node_load1 1 minute load average.
# TYPE node_load1 gauge
node_load1 0.72
# HELP node_load5 5 minute load average.
# TYPE node_load5 gauge
node_load5 0.58
# HELP node_load15 15 minute load average.
# TYPE node_load15 gauge
node_load15 0.41

# HELP node_filesystem_size_bytes Filesystem size in bytes.
# TYPE node_filesystem_size_bytes gauge
node_filesystem_size_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 1.05226698752e+11
# HELP node_filesystem_free_bytes Filesystem free space (for root).
# TYPE node_filesystem_free_bytes gauge
node_filesystem_free_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 5.2613349376e+10
# HELP node_filesystem_avail_bytes Filesystem space available to non-root.
# TYPE node_filesystem_avail_bytes gauge
node_filesystem_avail_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 4.7244640256e+10
# HELP node_filesystem_files Total inodes.
# TYPE node_filesystem_files gauge
node_filesystem_files{device="/dev/sda1",fstype="ext4",mountpoint="/"} 6.553600e+06
# HELP node_filesystem_files_free Free inodes.
# TYPE node_filesystem_files_free gauge
node_filesystem_files_free{device="/dev/sda1",fstype="ext4",mountpoint="/"} 6.124287e+06

# HELP node_boot_time_seconds Node boot time in seconds since epoch.
# TYPE node_boot_time_seconds gauge
node_boot_time_seconds 1.77307212e+09
# HELP node_time_seconds Current system time in seconds since epoch.
# TYPE node_time_seconds gauge
node_time_seconds 1.77333572e+09
```
