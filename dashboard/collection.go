package dashboard

import "github.com/timgluz/wasserspiegel/response"

type Collection struct {
	Items      []ListItem          `json:"items"`
	Pagination response.Pagination `json:"pagination"`
}

type ListItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	StationID    string `json:"station_id"`
	LanguageCode string `json:"language_code"`
	Timezone     string `json:"timezone"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

func mapDashboardToListItem(d *Dashboard) (ListItem, bool) {
	if d == nil {
		return ListItem{}, false
	}

	return ListItem{
		ID:           d.ID,
		Name:         d.Name,
		Description:  d.Description,
		StationID:    d.Station.ID,
		LanguageCode: d.LanguageCode,
		Timezone:     d.Timezone,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}, true
}

func NewDashboardListCollection(dashboards []Dashboard, pagination response.Pagination) *Collection {
	items := make([]ListItem, 0, len(dashboards))
	for _, d := range dashboards {
		if item, ok := mapDashboardToListItem(&d); ok {
			items = append(items, item)
		}
	}

	return &Collection{
		Items:      items,
		Pagination: pagination,
	}
}
