package merge

// CountryInfo describes one country/region entry from the source COUNTRY_MAP.
type CountryInfo struct {
	Code    string
	Name    string
	Emoji   string
	Aliases []string
}

// countryOrder preserves the declaration order of COUNTRY_MAP in merge.js.
// Order matters for the emoji-scan pass in extractCountry, which iterates the
// map and takes the first emoji found in the tag.
var countryOrder = []string{
	"HK", "CN", "US", "JP", "SG", "TW", "KR", "GB", "DE", "CA", "AU", "FR", "NL",
	"IN", "RU", "BR", "AR", "TR", "TH", "MY", "PH", "VN", "ID",
	// 欧洲国家
	"IT", "ES", "CH", "SE", "NO", "FI", "PL", "AT", "BE", "DK", "PT", "GR", "IE",
	"CZ", "RO", "UA", "LT", "LV", "EE", "BG", "HR", "SK", "SI", "HU",
	// 亚洲国家
	"IL", "AE", "SA", "KW", "PK", "BD", "KZ", "UZ",
	// 美洲国家
	"MX", "CL", "CO", "PE", "VE",
	// 非洲国家
	"ZA", "EG", "NG",
	// 大洋洲国家
	"NZ", "FJ",
}

// countryMap mirrors COUNTRY_MAP in merge.js: code -> {name, emoji, aliases}.
var countryMap = map[string]CountryInfo{
	"HK": {"HK", "Hong Kong", "🇭🇰", []string{"香港", "hongkong", "hk", "🇭🇰"}},
	"CN": {"CN", "China", "🇨🇳", []string{"中国", "china", "cn", "🇨🇳"}},
	"US": {"US", "United States", "🇺🇸", []string{"美国", "unitedstates", "america", "us", "🇺🇸"}},
	"JP": {"JP", "Japan", "🇯🇵", []string{"日本", "japan", "jp", "🇯🇵"}},
	"SG": {"SG", "Singapore", "🇸🇬", []string{"新加坡", "singapore", "sg", "🇸🇬"}},
	"TW": {"TW", "Taiwan", "🇹🇼", []string{"台湾", "taiwan", "tw", "🇹🇼"}},
	"KR": {"KR", "South Korea", "🇰🇷", []string{"韩国", "korea", "kr", "🇰🇷"}},
	"GB": {"GB", "United Kingdom", "🇬🇧", []string{"英国", "uk", "britain", "gb", "🇬🇧"}},
	"DE": {"DE", "Germany", "🇩🇪", []string{"德国", "germany", "de", "🇩🇪"}},
	"CA": {"CA", "Canada", "🇨🇦", []string{"加拿大", "canada", "ca", "🇨🇦"}},
	"AU": {"AU", "Australia", "🇦🇺", []string{"澳大利亚", "australia", "au", "🇦🇺"}},
	"FR": {"FR", "France", "🇫🇷", []string{"法国", "france", "fr", "🇫🇷"}},
	"NL": {"NL", "Netherlands", "🇳🇱", []string{"荷兰", "netherlands", "nl", "🇳🇱"}},
	"IN": {"IN", "India", "🇮🇳", []string{"印度", "india", "in", "🇮🇳"}},
	"RU": {"RU", "Russia", "🇷🇺", []string{"俄罗斯", "russia", "ru", "🇷🇺"}},
	"BR": {"BR", "Brazil", "🇧🇷", []string{"巴西", "brazil", "br", "🇧🇷"}},
	"AR": {"AR", "Argentina", "🇦🇷", []string{"阿根廷", "argentina", "ar", "🇦🇷"}},
	"TR": {"TR", "Turkey", "🇹🇷", []string{"土耳其", "turkey", "tr", "🇹🇷"}},
	"TH": {"TH", "Thailand", "🇹🇭", []string{"泰国", "thailand", "th", "🇹🇭"}},
	"MY": {"MY", "Malaysia", "🇲🇾", []string{"马来西亚", "malaysia", "my", "🇲🇾"}},
	"PH": {"PH", "Philippines", "🇵🇭", []string{"菲律宾", "philippines", "ph", "🇵🇭"}},
	"VN": {"VN", "Vietnam", "🇻🇳", []string{"越南", "vietnam", "vn", "🇻🇳"}},
	"ID": {"ID", "Indonesia", "🇮🇩", []string{"印度尼西亚", "印尼", "indonesia", "id", "🇮🇩"}},
	"IT": {"IT", "Italy", "🇮🇹", []string{"意大利", "italy", "it", "🇮🇹"}},
	"ES": {"ES", "Spain", "🇪🇸", []string{"西班牙", "spain", "es", "🇪🇸"}},
	"CH": {"CH", "Switzerland", "🇨🇭", []string{"瑞士", "switzerland", "ch", "🇨🇭"}},
	"SE": {"SE", "Sweden", "🇸🇪", []string{"瑞典", "sweden", "se", "🇸🇪"}},
	"NO": {"NO", "Norway", "🇳🇴", []string{"挪威", "norway", "no", "🇳🇴"}},
	"FI": {"FI", "Finland", "🇫🇮", []string{"芬兰", "finland", "fi", "🇫🇮"}},
	"PL": {"PL", "Poland", "🇵🇱", []string{"波兰", "poland", "pl", "🇵🇱"}},
	"AT": {"AT", "Austria", "🇦🇹", []string{"奥地利", "austria", "at", "🇦🇹"}},
	"BE": {"BE", "Belgium", "🇧🇪", []string{"比利时", "belgium", "be", "🇧🇪"}},
	"DK": {"DK", "Denmark", "🇩🇰", []string{"丹麦", "denmark", "dk", "🇩🇰"}},
	"PT": {"PT", "Portugal", "🇵🇹", []string{"葡萄牙", "portugal", "pt", "🇵🇹"}},
	"GR": {"GR", "Greece", "🇬🇷", []string{"希腊", "greece", "gr", "🇬🇷"}},
	"IE": {"IE", "Ireland", "🇮🇪", []string{"爱尔兰", "ireland", "ie", "🇮🇪"}},
	"CZ": {"CZ", "Czech Republic", "🇨🇿", []string{"捷克", "czech", "cz", "🇨🇿"}},
	"RO": {"RO", "Romania", "🇷🇴", []string{"罗马尼亚", "romania", "ro", "🇷🇴"}},
	"UA": {"UA", "Ukraine", "🇺🇦", []string{"乌克兰", "ukraine", "ua", "🇺🇦"}},
	"LT": {"LT", "Lithuania", "🇱🇹", []string{"立陶宛", "lithuania", "lt", "🇱🇹"}},
	"LV": {"LV", "Latvia", "🇱🇻", []string{"拉脱维亚", "latvia", "lv", "🇱🇻"}},
	"EE": {"EE", "Estonia", "🇪🇪", []string{"爱沙尼亚", "estonia", "ee", "🇪🇪"}},
	"BG": {"BG", "Bulgaria", "🇧🇬", []string{"保加利亚", "bulgaria", "bg", "🇧🇬"}},
	"HR": {"HR", "Croatia", "🇭🇷", []string{"克罗地亚", "croatia", "hr", "🇭🇷"}},
	"SK": {"SK", "Slovakia", "🇸🇰", []string{"斯洛伐克", "slovakia", "sk", "🇸🇰"}},
	"SI": {"SI", "Slovenia", "🇸🇮", []string{"斯洛文尼亚", "slovenia", "si", "🇸🇮"}},
	"HU": {"HU", "Hungary", "🇭🇺", []string{"匈牙利", "hungary", "hu", "🇭🇺"}},
	"IL": {"IL", "Israel", "🇮🇱", []string{"以色列", "israel", "il", "🇮🇱"}},
	"AE": {"AE", "United Arab Emirates", "🇦🇪", []string{"阿联酋", "uae", "emirates", "ae", "🇦🇪"}},
	"SA": {"SA", "Saudi Arabia", "🇸🇦", []string{"沙特", "沙特阿拉伯", "saudi", "sa", "🇸🇦"}},
	"KW": {"KW", "Kuwait", "🇰🇼", []string{"科威特", "kuwait", "kw", "🇰🇼"}},
	"PK": {"PK", "Pakistan", "🇵🇰", []string{"巴基斯坦", "pakistan", "pk", "🇵🇰"}},
	"BD": {"BD", "Bangladesh", "🇧🇩", []string{"孟加拉", "孟加拉国", "bangladesh", "bd", "🇧🇩"}},
	"KZ": {"KZ", "Kazakhstan", "🇰🇿", []string{"哈萨克斯坦", "kazakhstan", "kz", "🇰🇿"}},
	"UZ": {"UZ", "Uzbekistan", "🇺🇿", []string{"乌兹别克斯坦", "uzbekistan", "uz", "🇺🇿"}},
	"MX": {"MX", "Mexico", "🇲🇽", []string{"墨西哥", "mexico", "mx", "🇲🇽"}},
	"CL": {"CL", "Chile", "🇨🇱", []string{"智利", "chile", "cl", "🇨🇱"}},
	"CO": {"CO", "Colombia", "🇨🇴", []string{"哥伦比亚", "colombia", "co", "🇨🇴"}},
	"PE": {"PE", "Peru", "🇵🇪", []string{"秘鲁", "peru", "pe", "🇵🇪"}},
	"VE": {"VE", "Venezuela", "🇻🇪", []string{"委内瑞拉", "venezuela", "ve", "🇻🇪"}},
	"ZA": {"ZA", "South Africa", "🇿🇦", []string{"南非", "southafrica", "za", "🇿🇦"}},
	"EG": {"EG", "Egypt", "🇪🇬", []string{"埃及", "egypt", "eg", "🇪🇬"}},
	"NG": {"NG", "Nigeria", "🇳🇬", []string{"尼日利亚", "nigeria", "ng", "🇳🇬"}},
	"NZ": {"NZ", "New Zealand", "🇳🇿", []string{"新西兰", "newzealand", "nz", "🇳🇿"}},
	"FJ": {"FJ", "Fiji", "🇫🇯", []string{"斐济", "fiji", "fj", "🇫🇯"}},
}
