package channels

import (
	"context"
	"strings"
	"unicode"
)

// Topics каналов.
const (
	TopicNews     = "новости"
	TopicAquarium = "мой_морской_аквариум"
	TopicTrading  = "обучение_торговле_на_рынках"
)

var Topics = []string{TopicNews, TopicAquarium, TopicTrading}

func isAllowedTopic(topic string) bool {
	for _, t := range Topics {
		if t == topic {
			return true
		}
	}
	return false
}

var newsKeywords = []string{
	"news", "cnn", "bbc", "reuters", "fox news", "fox business", "nbc", "cbs", "abc news",
	"sky news", "euronews", "cgtn", "times of india", "firstpost", "oneindia", "newsx",
	"iltv", "baku tv", "dws news", "livenow", "newsnation", "news24", "nhk", "the weather channel",
	"new york times", "wall street journal", "washington post", "forbes", "the economist",
	"nikkei", "associated press", "the daily jagran", "cnbc", "bloomberg television",
	"bloomberg opinion", "bloomberg originals", "really american", "let be clear", "david pakman",
	"мировые дела", "взгляд из израиля", "китай в фокусе", "world affairs", "daily analysis",
	"ap agency", "federal reserve", "freedom on air", "maxim katz", "maksim katz",
	"the geo network", "техно прорыв",
}

var aquariumKeywords = []string{
	"reef", "coral", "aquarium", "aquatics", "marine", "saltwater", "fish", "tropic marin",
	"frag", "red sea", "brightwell", "tritonreef", "procoral", "melevsreef", "world wide corals",
	"reef builders", "inappropriate reefer", "saltwateraquarium", "fishofhex", "tidal gardens",
	"fragbox", "skipper reef", "american reef", "brian aquarium", "bahama llama coral",
	"queen city coral", "queen city reef", "thebioreef", "reef therapy", "detroit reef",
	"respect the reef", "piecesoftheocean", "green art marine", "top shelf aquatics",
	"marine reefer", "reefbum", "reef dork", "reef rookies", "brstv", "sac dep spa",
}

var otherKeywords = []string{
	"ai ", "ии", "neiro", "нейро", "backend", "development", "programming", "coding",
	"техно", "tech ", "web3", "crypto max", "rucars", "электромобил", "кик обзор",
	"мил-ман", "mechanical skills", "ilya lisov", "incubeai", "vladimir sukhov",
	"andrey ermolenko", "yevgeniy kovalenko", "marisol", "nesereznyi aytishnik",
	"freecodecamp", "the ai university", "ai pathways", "ai proryv", "ii ", "iiшный",
	"ii mastodont", "dresirovshik neirosetey", "arina x ai", "ivan_ai", "rixai",
	"ai automation", "backend development", "tech talk",
}

var tradingKeywords = []string{
	"trading", "trader", "trade", "finance", "investor", "investment", "investing",
	"gold", "silver", "bullion", "economics", "economic", "stock", "forex", "crypto",
	"schwab", "bank of america", "goldman", "kitco", "apmex", "sd bullion", "maneco",
	"the deep dive", "mind math money", "wall street truth", "bullion brief", "k-stocks",
	"solfer", "treider", "код денег", "деньги в банке", "sokol finance", "falcon finance",
	"finance mind", "finance focus", "economics help", "economix", "eu made simple",
	"global arc", "germania mint", "east money", "sina finance", "eastmoney",
	"wallstreet", "stocktwits",
	"трейдинг", "инвестиции", "инвестици", "финанс", "экономика",
	"брокер", "рынок", "акции", "облигации", "маркет",
}

var singleWordTradingKeywords = []string{
	"brianswart", "wallstreetsilver", "kitco news", "bullboard tv", "the market herald",
	"financial education", "the plain bagel", "graham Stephan live", " Andrei Jikh",
	"meet kevin", "robert reyvolt", "stuart mcnish", "craig c", "linda raynier",
	"peter l brandt", "peter brandt", "danielle di martino", "moneyball",
	"trading 212", "tradingwithrayner", "rayner", "trendSpider", "tradingview", "wysetrade",
}

var ambiguousKeywords = map[string]bool{
	"ии":    true,
	"ai":    true,
	"trade": true,
	"gold":  true,
	"stock": true,
	"fish":  true,
}

func containsKeyword(text, kw string) bool {
	word := strings.TrimSpace(kw)
	if !ambiguousKeywords[word] {
		return strings.Contains(text, kw)
	}
	return containsWord(text, word)
}

func containsWord(text, kw string) bool {
	allowedSuffixes := []string{"", "s", "es", "ed", "ing", "r", "rs", "er", "ers", "d"}

	for _, word := range splitWords(text) {
		if !strings.HasPrefix(word, kw) {
			continue
		}
		suffix := word[len(kw):]
		for _, allowed := range allowedSuffixes {
			if suffix == allowed {
				return true
			}
		}
	}
	return false
}

func splitWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func classifyChannel(channelName, description string) (string, bool) {
	text := strings.ToLower(channelName + " " + description)

	for _, kw := range newsKeywords {
		if containsKeyword(text, kw) {
			return TopicNews, true
		}
	}

	for _, kw := range aquariumKeywords {
		if containsKeyword(text, kw) {
			return TopicAquarium, false
		}
	}

	for _, kw := range otherKeywords {
		if containsKeyword(text, kw) {
			return TopicNews, true
		}
	}

	for _, kw := range tradingKeywords {
		if containsKeyword(text, kw) {
			return TopicTrading, false
		}
	}

	for _, kw := range singleWordTradingKeywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return TopicTrading, false
		}
	}

	return TopicNews, true
}

func classifyChannelWithLLM(llm *LLMClient, channelID, channelName, description string, videoTitles []string) (string, bool) {
	if llm != nil {
		topic, err := llm.Classify(context.Background(), channelID, channelName, description, videoTitles)
		if err == nil && topic != "" && isAllowedTopic(topic) {
			return topic, false
		}
	}

	topic, _ := classifyChannel(channelName, description)
	return topic, true
}
