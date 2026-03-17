package cleaner

func (c *Cleaner) AIClean(text string) (string, string) {
	if !c.cfg.AI.Enabled {
		return text, "rules"
	}

	cleaned, provider, err := c.aiClient.Clean(text)
	if err != nil || cleaned == "" {
		return text, "rules"
	}

	return cleaned, provider
}