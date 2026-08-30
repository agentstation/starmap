package modelsdev

import "github.com/agentstation/starmap/pkg/catalogs"

// ConvertToStarmapModel converts a models.dev model to a Starmap model. GitSource
// and HTTPSource share it to avoid duplicate conversion logic.
func ConvertToStarmapModel(mdModel Model) *catalogs.Model {
	model, err := mdModel.ToStarmapModel()
	if err != nil {
		return &catalogs.Model{ID: mdModel.ID, Name: mdModel.Name}
	}
	return model
}
