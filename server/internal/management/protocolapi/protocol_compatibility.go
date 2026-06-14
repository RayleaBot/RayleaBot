package protocolapi

import "github.com/RayleaBot/RayleaBot/server/internal/protocolcap"

func (s *ProtocolService) currentOneBot11ProtocolCompatibility() (oneBot11ProtocolCompatibilityResponse, error) {
	matrix := protocolcap.OneBot11CompatibilityMatrix()
	categories := make([]protocolCompatibilityCategoryResponse, 0, len(matrix.Categories))
	for _, category := range matrix.Categories {
		items := make([]protocolCompatibilityItemResponse, 0, len(category.Items))
		for _, item := range category.Items {
			items = append(items, protocolCompatibilityItemResponse{
				Key:   item.Key,
				Label: item.Label,
				Support: protocolCompatibilitySupportResponse{
					Standard:    item.Support.Standard,
					NapCat:      item.Support.NapCat,
					LuckyLillia: item.Support.LuckyLillia,
				},
				Summary: item.Summary,
			})
		}
		categories = append(categories, protocolCompatibilityCategoryResponse{
			Key:   category.Key,
			Title: category.Title,
			Items: items,
		})
	}

	return oneBot11ProtocolCompatibilityResponse{
		Protocol:   matrix.Protocol,
		Categories: categories,
	}, nil
}
