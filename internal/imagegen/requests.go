package imagegen

type Params struct {
	Model             string
	ForgePreset       string
	VAEAndTextModels  []string
	Prompt            string
	NegativePrompt    string
	Sampler           string
	Scheduler         string
	Steps             int
	Width             int
	Height            int
	CFGScale          float64
	Seed              int
	EnableHR          bool
	HRScale           float64
	HRUpscaler        string
	HRSecondPassSteps int
	HRCFGScale        float64
}

type Txt2ImgRequest struct {
	Params

	HRDenoisingStrength float64
}

type Img2ImgRequest struct {
	Params

	InitImage []byte
	Strength  float64
	Noise     float64
}
