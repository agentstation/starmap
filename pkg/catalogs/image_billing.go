package catalogs

// ImageBilling declares complete synchronous image charges for its operations.
// ImageUnit supplies the price for one image or UnitPixels * UnitIterations.
//
// Pixel-iteration charges use requested output dimensions and fixed iterations.
// DefaultImages, DefaultWidth, and DefaultHeight apply only to omitted inputs.
// Charges do not vary by quality or style. There is no separate input fee.
// It does not establish token consumption or grant an operation capability.
type ImageBilling struct {
	Basis          ImageBillingBasis   `json:"basis" yaml:"basis"`
	Operations     []ProviderOperation `json:"operations" yaml:"operations"`
	DefaultImages  int64               `json:"default_images" yaml:"default_images"`
	DefaultWidth   int64               `json:"default_width,omitempty" yaml:"default_width,omitempty"`
	DefaultHeight  int64               `json:"default_height,omitempty" yaml:"default_height,omitempty"`
	Iterations     int64               `json:"iterations,omitempty" yaml:"iterations,omitempty"`
	UnitPixels     int64               `json:"unit_pixels,omitempty" yaml:"unit_pixels,omitempty"`
	UnitIterations int64               `json:"unit_iterations,omitempty" yaml:"unit_iterations,omitempty"`
	RequestCharge  *bool               `json:"request_charge" yaml:"request_charge"`
}

// ImageBillingBasis names the unit priced by ImageUnit.
type ImageBillingBasis string

const (
	// ImageBillingImages charges each generated image.
	ImageBillingImages ImageBillingBasis = "images"
	// ImageBillingPixelIterations charges requested output pixels times fixed iterations.
	ImageBillingPixelIterations ImageBillingBasis = "pixel_iterations"
)

func (b *ImageBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.DefaultImages <= 0 || b.RequestCharge == nil {
		return billingValidationError("images", nil, "requires positive default_images and explicit request_charge")
	}
	if len(b.Operations) == 0 || len(b.Operations) > 2 {
		return billingValidationError("images.operations", b.Operations, "requires image generation or edit operations")
	}
	seen := make(map[ProviderOperation]bool, len(b.Operations))
	for _, op := range b.Operations {
		if (op != ProviderOperationImagesGenerations && op != ProviderOperationImagesEdits) || seen[op] {
			return billingValidationError("images.operations", op, "requires distinct image operations")
		}
		seen[op] = true
	}
	switch b.Basis {
	case ImageBillingImages:
		if b.DefaultWidth != 0 || b.DefaultHeight != 0 || b.Iterations != 0 || b.UnitPixels != 0 || b.UnitIterations != 0 {
			return billingValidationError("images", nil, "flat image billing cannot declare pixel or iteration factors")
		}
	case ImageBillingPixelIterations:
		if b.DefaultWidth <= 0 || b.DefaultHeight <= 0 || b.Iterations <= 0 || b.UnitPixels <= 0 || b.UnitIterations <= 0 {
			return billingValidationError("images", nil, "requires positive dimensions, iterations, and price units")
		}
	default:
		return billingValidationError("images.basis", b.Basis, "must be images or pixel_iterations")
	}
	return nil
}
