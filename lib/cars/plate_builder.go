package cars

import (
	"strings"
)

type characterType int

const (
	Number         characterType = 0x01
	LatinoCyrillic characterType = 0x02
	None           characterType = 0x04
)

func (c characterType) IsNumber() bool {
	return c&Number != 0
}

func (c characterType) IsLatinoCyrillic() bool {
	return c&LatinoCyrillic != 0
}

func (c characterType) IsNone() bool {
	return c&None != 0
}

type licensePlateType int

const (
	automobile licensePlateType = iota
	motorcycle
)

func (t licensePlateType) Next(index int) (next characterType) {
	for _, template := range licensePlateTypeTemplates[t] {
		if index == len(template) {
			next |= None
			continue
		}
		if index >= len(template) {
			continue
		}
		next |= template[index]
	}
	return next
}

var licensePlateTypeTemplates = map[licensePlateType][][]characterType{
	automobile: {
		{LatinoCyrillic, Number, Number, Number, LatinoCyrillic, LatinoCyrillic, Number, Number},
		{LatinoCyrillic, Number, Number, Number, LatinoCyrillic, LatinoCyrillic, Number, Number, Number}},
	motorcycle: {
		{Number, Number, Number, Number, LatinoCyrillic, LatinoCyrillic, Number, Number},
		{Number, Number, Number, Number, LatinoCyrillic, LatinoCyrillic, Number, Number, Number}},
}

const Numbers = "0123456789"
const ABCEHKMOPTXY = "ABCEHKMOPTXY"

func toCharacterTypeSlice(current string) []characterType {
	var out []characterType
	current = strings.ToUpper(current)
	for _, s := range current {
		var t characterType
		if strings.ContainsRune(ABCEHKMOPTXY, s) {
			t = LatinoCyrillic
		}
		if strings.ContainsRune(Numbers, s) {
			t = Number
		}
		out = append(out, t)
	}
	return out
}

func templateEquals(a, b []characterType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func possibleLicensePlateTypes(ctt []characterType) []licensePlateType {
	possibleTypes := map[licensePlateType]struct{}{}
	for tt, tepmlates := range licensePlateTypeTemplates {
		for _, template := range tepmlates {
			l := len(ctt)
			if l > len(template) {
				l = len(template)
			}
			if templateEquals(template[0:l], ctt) {
				possibleTypes[tt] = struct{}{}
			}
		}
	}
	var out []licensePlateType
	for lpt := range possibleTypes {
		out = append(out, lpt)
	}
	return out
}

func NextCharacterType(current string) (next characterType) {
	if current == "" {
		return Number | LatinoCyrillic
	}
	for _, plpt := range possibleLicensePlateTypes(toCharacterTypeSlice(current)) {
		next |= plpt.Next(len(current))
	}
	return next
}

func LicensePlateHints(plate string) string {
	switch len(plate) {
	case 0:
		return "Начнём с первого символа."
	case 1:
		return "Теперь 3 цифры."
	case 2:
		return "Ещё две цифры."
	case 3:
		return "И ещё одна цифра"
	case 4:
		return "Две последние буквы"
	case 5:
		return "И ещё одна"
	case 6:
		return "Теперь номер региона. 96?"
	case 7:
		if plate[6] == '7' {
			return "Москва? Питер?"
		}
	}
	return ""
}
