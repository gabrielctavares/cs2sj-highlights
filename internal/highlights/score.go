package highlights

import (
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

var factorLabels = map[string]string{
	"kill_count": "sequência de eliminações", "assist": "assistências", "flash_assist": "flash assists",
	"headshot": "headshot", "wallbang": "wallbang", "no_scope": "no-scope", "smoke": "kill através da smoke",
	"blind": "kill enquanto estava cego", "long_range": "longa distância", "rare_weapon": "arma de alta dificuldade",
	"fast_sequence": "sequência rápida", "clutch": "clutch", "low_hp": "pouco HP", "won_round": "round vencido",
	"opening": "entry kill", "trade": "trade", "match_point": "match point", "overtime": "overtime",
	"match_end": "fim da partida", "multi_impact": "alto impacto no round",
}

func Evaluate(candidate model.Highlight, rules Rules) model.Highlight {
	individual := make(map[string]int)
	base := map[int]int{0: 0, 1: 20, 2: 38, 3: 62, 4: 80}
	killPoints, ok := base[candidate.Context.KillCount]
	if !ok {
		killPoints = 95
	}
	addFactor(individual, "kill_count", killPoints)
	addFactor(individual, "assist", min(candidate.Context.AssistCount*4, 8))
	addFactor(individual, "flash_assist", min(candidate.Context.FlashAssistCount*18, 36))

	headshots := 0
	wallbang, noScope, smoke, blind, longRange := false, false, false, false, false
	rareWeapon := 0
	for _, kill := range candidate.Actions {
		if !samePlayer(kill.Killer, candidate.Player) {
			continue
		}
		if kill.IsHeadshot {
			headshots++
		}
		wallbang = wallbang || kill.PenetratedObjects > 0
		noScope = noScope || kill.NoScope
		smoke = smoke || kill.ThroughSmoke
		blind = blind || kill.AttackerBlind
		longRange = longRange || kill.DistanceKnown && kill.Distance >= rules.LongRangeUnits
		weapon := strings.ToLower(kill.Weapon)
		switch {
		case strings.Contains(weapon, "knife") || strings.Contains(weapon, "bayonet"):
			rareWeapon = max(rareWeapon, 20)
		case strings.Contains(weapon, "zeus"):
			rareWeapon = max(rareWeapon, 18)
		case strings.Contains(weapon, "deagle") || strings.Contains(weapon, "desert eagle"):
			rareWeapon = max(rareWeapon, 6)
		}
	}
	addFactor(individual, "headshot", min(headshots*8, 16))
	addBoolFactor(individual, "wallbang", wallbang, 15)
	addBoolFactor(individual, "no_scope", noScope, 18)
	addBoolFactor(individual, "smoke", smoke, 18)
	addBoolFactor(individual, "blind", blind, 12)
	addBoolFactor(individual, "long_range", longRange, 10)
	addFactor(individual, "rare_weapon", rareWeapon)
	addBoolFactor(individual, "fast_sequence", candidate.Context.FastSequence, 10)
	if candidate.Context.ClutchOpponents > 0 {
		addFactor(individual, "clutch", 12+6*candidate.Context.ClutchOpponents)
	}
	addBoolFactor(individual, "low_hp", candidate.Context.LowestKillerHealth > 0 && candidate.Context.LowestKillerHealth <= rules.LowHPThreshold, 10)

	individualEvaluation := buildEvaluation(individual, candidateConfidence(candidate))
	editorial := map[string]int{"technical_merit": individualEvaluation.Score * 70 / 100}
	addFactor(editorial, "assist", min(candidate.Context.AssistCount*4, 8))
	addFactor(editorial, "flash_assist", min(candidate.Context.FlashAssistCount*10, 20))
	addBoolFactor(editorial, "won_round", candidate.Context.WonRound, 10)
	addBoolFactor(editorial, "opening", candidate.Context.OpeningKill, 8)
	addFactor(editorial, "trade", min(candidate.Context.TradeKills*5, 10))
	if candidate.Context.ClutchOpponents > 0 {
		addFactor(editorial, "clutch", 15+5*candidate.Context.ClutchOpponents)
	}
	addBoolFactor(editorial, "match_point", candidate.Context.MatchPoint, 15)
	addBoolFactor(editorial, "overtime", candidate.Context.Overtime > 0, 10)
	addBoolFactor(editorial, "match_end", candidate.Context.MatchEnd, 8)
	addBoolFactor(editorial, "multi_impact", candidate.Context.KillCount >= 3, 10)
	editorialEvaluation := buildEvaluation(editorial, individualEvaluation.Confidence)

	candidate.Individual = individualEvaluation
	candidate.Editorial = editorialEvaluation
	candidate.Priority = editorialEvaluation.Score
	return candidate
}

func addFactor(factors map[string]int, code string, points int) {
	if points > 0 {
		factors[code] += points
	}
}

func addBoolFactor(factors map[string]int, code string, condition bool, points int) {
	if condition {
		addFactor(factors, code, points)
	}
}

func buildEvaluation(values map[string]int, confidence model.Confidence) model.Evaluation {
	factors := make([]model.ScoreFactor, 0, len(values))
	total := 0
	for code, points := range values {
		if points <= 0 {
			continue
		}
		total += points
		factors = append(factors, model.ScoreFactor{Code: code, Points: points})
	}
	total = min(total, 100)
	sort.SliceStable(factors, func(i, j int) bool {
		if factors[i].Points != factors[j].Points {
			return factors[i].Points > factors[j].Points
		}
		return factors[i].Code < factors[j].Code
	})
	labels := make([]string, 0, 3)
	for _, factor := range factors {
		if factor.Code == "technical_merit" {
			continue
		}
		if label := factorLabels[factor.Code]; label != "" {
			labels = append(labels, label)
		}
		if len(labels) == 3 {
			break
		}
	}
	return model.Evaluation{Score: total, Confidence: confidence, Factors: factors, Explanation: strings.Join(labels, ", ")}
}

func candidateConfidence(candidate model.Highlight) model.Confidence {
	if candidate.Context.KillCount == 0 {
		if candidate.Context.AssistCount > 0 {
			return model.ConfidenceMedium
		}
		return model.ConfidenceLow
	}
	optionalMissing := false
	for _, kill := range candidate.Actions {
		if !samePlayer(kill.Killer, candidate.Player) {
			continue
		}
		killerID, killerOK := candidateIdentity(kill.Killer)
		victimID, victimOK := candidateIdentity(kill.Victim)
		if !killerOK || !victimOK || killerID == "" || victimID == "" || kill.Weapon == "" || strings.EqualFold(kill.Weapon, "unknown") {
			return model.ConfidenceLow
		}
		optionalMissing = optionalMissing || !kill.DistanceKnown
	}
	if optionalMissing {
		return model.ConfidenceMedium
	}
	return model.ConfidenceHigh
}
