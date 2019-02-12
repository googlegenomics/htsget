package model

type HTSGetResponse struct {
	Htsget struct {
		Format string `json:"format"`
		Urls   []URL  `json:"urls"`
	} `json:"htsget"`
}

type URL struct {
	Url string `json:"url"`
}
