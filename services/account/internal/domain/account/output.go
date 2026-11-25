package account

type OutputSource struct {
	Key   string `json:"scope_key"`
	Epoch string `json:"epoch"`
}

type OutputBinding struct {
	Sources map[string]string
	Invalid bool
}
