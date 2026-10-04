package campaigns

import (
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestBuildCampaignStructureCountsUniqueDestinations(t *testing.T) {
	items := buildCampaignStructure([]models.Stream{
		{
			ID: "str_a1", CampaignID: "cmp_a",
			Distribution: models.Distribution{Destinations: []models.WeightedTarget{
				{DestinationID: "dst_one"},
				{DestinationID: "dst_two"},
			}},
		},
		{
			ID: "str_a2", CampaignID: "cmp_a",
			Distribution: models.Distribution{Destinations: []models.WeightedTarget{
				{DestinationID: "dst_one"},
			}},
		},
		{ID: "str_b1", CampaignID: "cmp_b"},
	})
	if len(items) != 2 || items[0].CampaignID != "cmp_a" ||
		items[0].StreamCount != 2 || items[0].DestinationCount != 2 ||
		items[1].CampaignID != "cmp_b" || items[1].StreamCount != 1 ||
		items[1].DestinationCount != 0 {
		t.Fatalf("unexpected campaign structure: %#v", items)
	}
}
