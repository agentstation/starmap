package config

import (
	"strconv"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// Canonical retention settings control runtime maintenance independently of source refresh.
const (
	// RetentionEnabled selects automatic retention maintenance.
	RetentionEnabled = Prefix + "CATALOG_RETENTION_ENABLED"
	// RetentionInterval sets the positive maintenance interval.
	RetentionInterval = Prefix + "CATALOG_RETENTION_INTERVAL"
	// RetentionMaxGenerations sets the retained generation count target.
	RetentionMaxGenerations = Prefix + "CATALOG_RETENTION_MAX_GENERATIONS"
	// RetentionMaxBytes sets the retained generation byte target.
	RetentionMaxBytes = Prefix + "CATALOG_RETENTION_MAX_BYTES"
	// RetentionScanEntries bounds one maintenance scan.
	RetentionScanEntries = Prefix + "CATALOG_RETENTION_SCAN_ENTRIES"
	// RetentionInputMaxBytes bounds raw input bytes captured for collection.
	RetentionInputMaxBytes = Prefix + "CATALOG_RETENTION_INPUT_MAX_BYTES"
)

func retentionSettings() []setting {
	return []setting{
		{name: RetentionEnabled, flag: "catalog-retention-enabled", apply: boolOption(RetentionEnabled, runtime.WithRetentionEnabled)},
		{name: RetentionInterval, flag: "catalog-retention-interval", apply: durationOption(RetentionInterval, runtime.WithRetentionInterval)},
		{name: RetentionMaxGenerations, flag: "catalog-retention-max-generations", apply: intOption(RetentionMaxGenerations, runtime.WithRetentionMaxGenerations)},
		{name: RetentionMaxBytes, flag: "catalog-retention-max-bytes", apply: retentionBytesOption(RetentionMaxBytes, runtime.WithRetentionMaxBytes)},
		{name: RetentionScanEntries, flag: "catalog-retention-scan-entries", apply: retentionEntriesOption},
		{name: RetentionInputMaxBytes, flag: "catalog-retention-input-max-bytes", apply: retentionBytesOption(RetentionInputMaxBytes, runtime.WithRetentionInputMaxBytes)},
	}
}

func retentionBytesOption(name string, option func(int64) runtime.Option) func(string) (runtime.Option, error) {
	return func(value string) (runtime.Option, error) {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			return nil, &errors.ValidationError{Field: name, Message: "must be a positive 64-bit byte count"}
		}
		return option(parsed), nil
	}
}

func retentionEntriesOption(value string) (runtime.Option, error) {
	entries, err := strconv.Atoi(value)
	if err != nil || entries < 1 || entries > storage.MaxRetentionScanEntries {
		return nil, &errors.ValidationError{Field: RetentionScanEntries, Message: "must be between 1 and 100000 entries"}
	}
	return runtime.WithRetentionScanEntries(entries), nil
}

func describeRetention(d *Descriptor) {
	policy := runtime.DefaultRetentionPolicy()
	switch d.Name {
	case RetentionEnabled:
		d.Description = "Enables automatic retention maintenance. Offline mode and generation pins still permit local cleanup."
		d.Type, d.Default = BooleanValue, strconv.FormatBool(policy.Enabled)
	case RetentionInterval:
		d.Description = "Sets the positive automatic collection interval. Disable scheduling with catalog_retention_enabled."
		d.Type, d.Unit, d.Default = DurationValue, "duration", policy.Interval.String()
	case RetentionMaxGenerations:
		d.Description = "Sets the retained generation count target. Required generations can exceed this limit."
		d.Type, d.Unit, d.Default = IntegerValue, "generations", strconv.Itoa(policy.MaxGenerations)
	case RetentionMaxBytes:
		d.Description = "Sets the retained generation byte target. Required generations can exceed this limit."
		d.Type, d.Unit, d.Default = IntegerValue, "bytes", strconv.FormatInt(policy.MaxBytes, 10)
	case RetentionScanEntries:
		d.Description = "Bounds one collection scan to at most 100000 entries. An incomplete scan refuses deletion."
		d.Type, d.Unit, d.Default = IntegerValue, "entries", strconv.Itoa(policy.ScanEntries)
	case RetentionInputMaxBytes:
		d.Description = "Bounds raw retained input bytes captured during one scan. It excludes decoder and filesystem overhead."
		d.Type, d.Unit, d.Default = IntegerValue, "bytes", strconv.FormatInt(policy.InputMaxBytes, 10)
	}
}
