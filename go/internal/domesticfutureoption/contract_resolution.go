package domesticfutureoption

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
)

func (s *Service) ResolveNearMonthKOSPI200Futures(ctx context.Context, businessDate string) (*ResolvedContract, error) {
	override := strings.TrimSpace(os.Getenv(quadWitchingFuturesCodeEnvKey))
	if override != "" {
		return &ResolvedContract{
			BusinessDate: normalizeBusinessDate(businessDate),
			Source:       "env",
			Record: MasterRecord{
				ShortCode: override,
				Name:      "ENV override",
			},
		}, nil
	}

	records, err := s.LoadIndexFutureMaster(ctx, businessDate)
	if err != nil {
		return nil, err
	}

	record, err := selectNearMonthKOSPI200Future(records)
	if err != nil {
		return nil, err
	}

	cachePath := resolveIndexFutureMasterCachePath(getOrDefaultEnv(indexFutureMasterCacheEnvKey, defaultIndexFutureMasterCache), businessDate)
	return &ResolvedContract{
		BusinessDate:    normalizeBusinessDate(businessDate),
		Source:          "master",
		MasterCachePath: cachePath,
		MasterJSONPath:  resolveIndexFutureMasterJSONPath(cachePath),
		Record:          record,
	}, nil
}

func selectNearMonthKOSPI200Future(records []MasterRecord) (MasterRecord, error) {
	candidates := make([]MasterRecord, 0, len(records))
	for _, record := range records {
		if record.ShortCode == "" {
			continue
		}
		if strings.TrimSpace(record.InfoType) != "1" {
			continue
		}
		if !strings.HasPrefix(record.ShortCode, "101") && !strings.Contains(strings.ToUpper(record.UnderlyingName), "KOSPI") {
			continue
		}
		candidates = append(candidates, record)
	}

	if len(candidates) == 0 {
		return MasterRecord{}, errors.New("unable to find KOSPI200 index futures contract in master file")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftScore := futureCandidateScore(candidates[i])
		rightScore := futureCandidateScore(candidates[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if candidates[i].MonthClassCode != candidates[j].MonthClassCode {
			return candidates[i].MonthClassCode < candidates[j].MonthClassCode
		}
		return candidates[i].ShortCode < candidates[j].ShortCode
	})

	return candidates[0], nil
}

func futureCandidateScore(record MasterRecord) int {
	score := 0
	if record.InfoType == "1" {
		score += 100
	}
	if strings.HasPrefix(record.ShortCode, "101") {
		score += 50
	}
	switch record.MonthClassCode {
	case "1":
		score += 30
	case "2":
		score += 20
	case "3":
		score += 10
	}
	if strings.Contains(strings.ToUpper(record.UnderlyingName), "KOSPI") {
		score += 5
	}
	return score
}
