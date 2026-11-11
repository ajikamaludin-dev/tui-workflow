package editor

func TriggerAutoSave(tm *TabManager) (int, error) {
	if tm == nil {
		return 0, nil
	}

	saved := 0
	for i, t := range tm.Tabs {
		if t.IsDirty() {
			if err := tm.SaveTab(i); err != nil {
				return saved, err
			}
			saved++
		}
	}
	return saved, nil
}
