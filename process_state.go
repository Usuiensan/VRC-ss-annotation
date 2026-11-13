package main

const vrchatWorldURLPrefix = "https://vrchat.com/home/world/"

func worldURLForID(worldID string) string {
	if worldID == "" {
		return ""
	}
	return vrchatWorldURLPrefix + worldID
}

// ProcessingStatus values are persisted in watch-state.jsonl and therefore
// must remain stable. Keeping them in one place prevents state transitions
// from silently diverging between the watcher, retry logic, and exporters.
const (
	processingStatusPending = "pending"
	processingStatusSuccess = "success"
	processingStatusFailed  = "failed"
	processingStatusSkipped = "skipped"
	processingStatusDuplicate = "duplicate"
)

// processStateForRecord copies the domain data into the persisted state model.
// Keeping this mapping separate makes new metadata fields easier to add without
// changing the export orchestration itself.
func processStateForRecord(record PhotoRecord) ProcessStateEntry {
	entry := stateEntryForPath(record.SourcePath)
	entry.SourceType = record.SourceType
	entry.WorldID = record.WorldID
	entry.WorldName = record.WorldName
	entry.InstanceID = record.InstanceID
	entry.InstanceType = record.InstanceType
	entry.PresentUsers = record.PresentUsers
	entry.WorldFilledFromLog = record.WorldFilledFromLog
	return entry
}

func skippedProcessState(path string) ProcessStateEntry {
	entry := stateEntryForPath(path)
	entry.EagleStatus = processingStatusSkipped
	entry.AmazonStatus = processingStatusSkipped
	return entry
}

func processEagleExport(entry *ProcessStateEntry, record PhotoRecord) {
	if !eagleEnabled() {
		entry.EagleStatus = processingStatusSkipped
		return
	}
	idx, err := getEagleHashIndex()
	if err != nil {
		entry.EagleStatus = processingStatusFailed
		entry.Error = joinErrors(entry.Error, "eagle: "+err.Error())
		return
	}
	result, err := idx.Import(record)
	entry.ContentSHA256 = result.Hash
	entry.EagleItemID = result.ItemID
	if err != nil {
		entry.EagleStatus = processingStatusFailed
		entry.Error = joinErrors(entry.Error, "eagle: "+err.Error())
		return
	}
	if result.Duplicate {
		entry.EagleStatus = processingStatusDuplicate
		appendLog("Eagle登録済みのためインポートを除外: " + record.SourcePath)
		return
	}
	entry.EagleStatus = processingStatusSuccess
}
