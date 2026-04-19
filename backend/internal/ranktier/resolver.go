package ranktier

import "speedhosting/backend/internal/models"

const InitialMMR = 400

type definition struct {
	minimumMMR int
	tier       models.RankedTier
}

var definitions = []definition{
	{
		minimumMMR: 800,
		tier: models.RankedTier{
			TierKey:      "champion",
			TierName:     "Champion",
			TierOrder:    7,
			TierColorHex: "#F59E0B",
			TierTag:      "[Champion]",
		},
	},
	{
		minimumMMR: 700,
		tier: models.RankedTier{
			TierKey:      "elite",
			TierName:     "Elite",
			TierOrder:    6,
			TierColorHex: "#EF4444",
			TierTag:      "[Elite]",
		},
	},
	{
		minimumMMR: 600,
		tier: models.RankedTier{
			TierKey:      "diamond",
			TierName:     "Diamond",
			TierOrder:    5,
			TierColorHex: "#60A5FA",
			TierTag:      "[Diamond]",
		},
	},
	{
		minimumMMR: 500,
		tier: models.RankedTier{
			TierKey:      "platinum",
			TierName:     "Platinum",
			TierOrder:    4,
			TierColorHex: "#2DD4BF",
			TierTag:      "[Platinum]",
		},
	},
	{
		minimumMMR: 400,
		tier: models.RankedTier{
			TierKey:      "gold",
			TierName:     "Gold",
			TierOrder:    3,
			TierColorHex: "#D4AF37",
			TierTag:      "[Gold]",
		},
	},
	{
		minimumMMR: 300,
		tier: models.RankedTier{
			TierKey:      "silver",
			TierName:     "Silver",
			TierOrder:    2,
			TierColorHex: "#94A3B8",
			TierTag:      "[Silver]",
		},
	},
	{
		minimumMMR: -1 << 30,
		tier: models.RankedTier{
			TierKey:      "bronze",
			TierName:     "Bronze",
			TierOrder:    1,
			TierColorHex: "#92400E",
			TierTag:      "[Bronze]",
		},
	},
}

func Resolve(mmr int) models.RankedTier {
	for _, definition := range definitions {
		if mmr >= definition.minimumMMR {
			return definition.tier
		}
	}

	return definitions[len(definitions)-1].tier
}
