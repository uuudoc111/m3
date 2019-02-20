
	defaultFilePathPrefix                = "/var/lib/m3db"
	defaultWriteBufferSize               = 65536
	defaultDataReadBufferSize            = 65536
	defaultInfoReadBufferSize            = 128
	defaultSeekReadBufferSize            = 4096
	defaultThroughputLimitMbps           = 100.0
	defaultThroughputCheckEvery          = 128
	defaultForceIndexSummariesMmapMemory = false
	defaultForceBloomFilterMmapMemory    = false
	FilePathPrefix *string `yaml:"filePathPrefix"`
	WriteBufferSize *int `yaml:"writeBufferSize"`
	DataReadBufferSize *int `yaml:"dataReadBufferSize"`
	InfoReadBufferSize *int `yaml:"infoReadBufferSize"`
	SeekReadBufferSize *int `yaml:"seekReadBufferSize"`
	ThroughputLimitMbps *float64 `yaml:"throughputLimitMbps"`
	ThroughputCheckEvery *int `yaml:"throughputCheckEvery"`
	ForceIndexSummariesMmapMemory *bool `yaml:"force_index_summaries_mmap_memory"`
	ForceBloomFilterMmapMemory *bool `yaml:"force_bloom_filter_mmap_memory"`
}

// Validate validates the Filesystem configuration. We use this method to validate
// fields where the validator package falls short.
func (f FilesystemConfiguration) Validate() error {
	if f.WriteBufferSize != nil && *f.WriteBufferSize < 1 {
		return fmt.Errorf(
			"fs writeBufferSize is set to: %d, but must be at least 1",
			*f.WriteBufferSize)
	}

	if f.DataReadBufferSize != nil && *f.DataReadBufferSize < 1 {
		return fmt.Errorf(
			"fs dataReadBufferSize is set to: %d, but must be at least 1",
			*f.DataReadBufferSize)
	}

	if f.InfoReadBufferSize != nil && *f.InfoReadBufferSize < 1 {
		return fmt.Errorf(
			"fs infoReadBufferSize is set to: %d, but must be at least 1",
			*f.InfoReadBufferSize)
	}

	if f.SeekReadBufferSize != nil && *f.SeekReadBufferSize < 1 {
		return fmt.Errorf(
			"fs seekReadBufferSize is set to: %d, but must be at least 1",
			*f.SeekReadBufferSize)
	}

	if f.ThroughputLimitMbps != nil && *f.ThroughputLimitMbps < 1 {
		return fmt.Errorf(
			"fs throughputLimitMbps is set to: %f, but must be at least 1",
			*f.ThroughputLimitMbps)
	}

	if f.ThroughputCheckEvery != nil && *f.ThroughputCheckEvery < 1 {
		return fmt.Errorf(
			"fs throughputCheckEvery is set to: %d, but must be at least 1",
			*f.ThroughputCheckEvery)
	}

	return nil
}

// FilePathPrefixOrDefault returns the configured file path prefix if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) FilePathPrefixOrDefault() string {
	if f.FilePathPrefix != nil {
		return *f.FilePathPrefix
	}

	return defaultFilePathPrefix
}

// WriteBufferSizeOrDefault returns the configured write buffer size if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) WriteBufferSizeOrDefault() int {
	if f.WriteBufferSize != nil {
		return *f.WriteBufferSize
	}

	return defaultWriteBufferSize
}

// DataReadBufferSizeOrDefault returns the configured data read buffer size if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) DataReadBufferSizeOrDefault() int {
	if f.DataReadBufferSize != nil {
		return *f.DataReadBufferSize
	}

	return defaultDataReadBufferSize
}

// InfoReadBufferSizeOrDefault returns the configured info read buffer size if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) InfoReadBufferSizeOrDefault() int {
	if f.InfoReadBufferSize != nil {
		return *f.InfoReadBufferSize
	}

	return defaultInfoReadBufferSize
}

// SeekReadBufferSizeOrDefault returns the configured seek read buffer size if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) SeekReadBufferSizeOrDefault() int {
	if f.SeekReadBufferSize != nil {
		return *f.SeekReadBufferSize
	}

	return defaultSeekReadBufferSize
}

// ThroughputLimitMbpsOrDefault returns the configured throughput limit mbps if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) ThroughputLimitMbpsOrDefault() float64 {
	if f.ThroughputLimitMbps != nil {
		return *f.ThroughputLimitMbps
	}

	return defaultThroughputLimitMbps
}

// ThroughputCheckEveryOrDefault returns the configured throughput check every value if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) ThroughputCheckEveryOrDefault() int {
	if f.ThroughputCheckEvery != nil {
		return *f.ThroughputCheckEvery
	}

	return defaultThroughputCheckEvery
}

// MmapConfigurationOrDefault returns the configured mmap configuration if configured, or a
// default value otherwise.
func (f FilesystemConfiguration) MmapConfigurationOrDefault() MmapConfiguration {
	if f.Mmap == nil {
		return DefaultMmapConfiguration()
	}
	return *f.Mmap
}

// ForceIndexSummariesMmapMemoryOrDefault returns the configured value for forcing the summaries
// mmaps into anonymous region in memory if configured, or a default value otherwise.
func (f FilesystemConfiguration) ForceIndexSummariesMmapMemoryOrDefault() bool {
	if f.ForceIndexSummariesMmapMemory != nil {
		return *f.ForceIndexSummariesMmapMemory
	}

	return defaultForceIndexSummariesMmapMemory
}

// ForceBloomFilterMmapMemoryOrDefault returns the configured value for forcing the bloom
// filter mmaps into anonymous region in memory if configured, or a default value otherwise.
func (f FilesystemConfiguration) ForceBloomFilterMmapMemoryOrDefault() bool {
	if f.ForceBloomFilterMmapMemory != nil {
		return *f.ForceBloomFilterMmapMemory
	}

	return defaultForceBloomFilterMmapMemory
func (f FilesystemConfiguration) ParseNewFileMode() (os.FileMode, error) {
	if f.NewFileMode == nil {
	str := *f.NewFileMode
func (f FilesystemConfiguration) ParseNewDirectoryMode() (os.FileMode, error) {
	if f.NewDirectoryMode == nil {
	str := *f.NewDirectoryMode