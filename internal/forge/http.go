package forge

// ImagesResponse represents the response from the Forge API for image requests.
//
// Even though it returns multiple images, we only ever deal with one output at a time.
type imagesResponse struct {
	Images []string `json:"images"`
}
