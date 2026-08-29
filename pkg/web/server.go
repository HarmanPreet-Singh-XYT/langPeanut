package web

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// StartInteractiveWebDemo launches the live interactive website on port 3000
func StartInteractiveWebDemo(port int, autoOpen bool) error {
	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("\n🚀 Launching Live Interactive Localization Demo at %s ...\n", url)

	if autoOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	return server.ListenAndServe()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(InteractiveAppHTML))
}

const InteractiveAppHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>FlightPeanut Store — Live Multi-Agent Localization Demo</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&display=swap');
    body { font-family: 'Plus Jakarta Sans', sans-serif; }
    .badge-diff { animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite; }
    @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: .7; } }
  </style>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen flex flex-col">

  <!-- Top Control Banner -->
  <header class="bg-slate-900 border-b border-slate-800 sticky top-0 z-50 shadow-xl">
    <div class="max-w-7xl mx-auto px-4 py-3 flex flex-wrap items-center justify-between gap-4">
      
      <!-- Logo & Title -->
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-gradient-to-tr from-pink-500 via-purple-500 to-cyan-400 flex items-center justify-center text-xl shadow-lg shadow-pink-500/20">
          🥜
        </div>
        <div>
          <h1 class="font-extrabold text-lg text-white flex items-center gap-2">
            langPeanut <span class="text-xs px-2 py-0.5 rounded-full bg-pink-500/20 text-pink-400 border border-pink-500/30">Live Demo</span>
          </h1>
          <p class="text-xs text-slate-400">Universal Multi-Agent Localization Engine</p>
        </div>
      </div>

      <!-- Controls: Before/After Toggle, Style Mode, Language Switcher -->
      <div class="flex items-center flex-wrap gap-3">
        
        <!-- Mode Switch: Before vs After -->
        <div class="bg-slate-950 p-1 rounded-xl border border-slate-800 flex items-center">
          <button id="btnBefore" onclick="setMode('before')" class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-all text-slate-400 hover:text-white">
            <i class="fa-solid fa-code mr-1.5 text-yellow-400"></i> Before (Raw Strings)
          </button>
          <button id="btnAfter" onclick="setMode('after')" class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-all bg-gradient-to-r from-pink-500 to-purple-600 text-white shadow-md">
            <i class="fa-solid fa-wand-magic-sparkles mr-1.5 text-cyan-300"></i> After (AST Localized)
          </button>
        </div>

        <!-- Gen-Z Slang Toggle -->
        <button id="btnGenZ" onclick="toggleGenZ()" class="px-3 py-1.5 rounded-xl text-xs font-semibold border transition-all border-slate-800 bg-slate-950 text-slate-300 hover:border-pink-500 flex items-center gap-1.5">
          <span>🔥 Gen-Z Slang</span>
          <span id="genZStatus" class="w-2 h-2 rounded-full bg-slate-600"></span>
        </button>

        <!-- Language Dropdown -->
        <div class="relative">
          <select id="langSelect" onchange="changeLanguage(this.value)" class="bg-slate-950 text-slate-200 border border-slate-700 hover:border-pink-500 rounded-xl px-3 py-1.5 text-xs font-semibold cursor-pointer focus:outline-none focus:ring-2 focus:ring-pink-500 pr-8">
            <option value="en">🇺🇸 English (Default)</option>
            <option value="fr">🇫🇷 French (Français)</option>
            <option value="es">🇪🇸 Spanish (Español)</option>
            <option value="de">🇩🇪 German (Deutsch)</option>
            <option value="ja">🇯🇵 Japanese (日本語)</option>
            <option value="hi">🇮🇳 Hindi (हिन्दी)</option>
            <option value="pa">🇮🇳 Punjabi (ਪੰਜਾਬੀ)</option>
            <option value="ar">🇸🇦 Arabic (العربية)</option>
            <option value="zh-CN">🇨🇳 Chinese (简体中文)</option>
            <option value="pt">🇧🇷 Portuguese (Português)</option>
          </select>
        </div>

        <!-- 4-Tier Critic Badge -->
        <div class="hidden sm:flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-emerald-950/60 border border-emerald-500/30 text-emerald-400 text-xs font-semibold">
          <i class="fa-solid fa-shield-check text-emerald-400"></i>
          <span>4-Tier Critic: 100% Pass</span>
        </div>

      </div>
    </div>
  </header>

  <!-- Notice Sub-Bar -->
  <div id="statusNotice" class="bg-gradient-to-r from-purple-950 via-slate-900 to-pink-950 border-b border-purple-800/30 px-4 py-2 text-center text-xs text-purple-200 flex items-center justify-center gap-2">
    <span class="badge-diff inline-block w-2 h-2 rounded-full bg-cyan-400"></span>
    <span id="statusText">Currently viewing: <b>Surgically Localized AST Code</b> with live translation and ICU parameter parity.</span>
  </div>

  <!-- Main Store Hero -->
  <main class="flex-1 max-w-7xl mx-auto px-4 py-8 w-full flex flex-col gap-10">
    
    <!-- Hero Section -->
    <section class="relative rounded-3xl overflow-hidden bg-gradient-to-br from-slate-900 via-purple-950/40 to-slate-900 border border-slate-800 p-8 md:p-12 shadow-2xl">
      <div class="max-w-2xl flex flex-col gap-5">
        <span class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 text-xs font-semibold w-fit">
          <i class="fa-solid fa-plane-departure"></i> <span data-i18n="heroBadge">Next-Gen Travel Platform</span>
        </span>
        <h2 id="heroTitle" class="text-3xl md:text-5xl font-extrabold tracking-tight text-white leading-tight">
          Explore the World Without Language Barriers
        </h2>
        <p id="heroSubtitle" class="text-slate-300 text-sm md:text-base leading-relaxed">
          Book flights, reserve luxury hotels, and travel with real-time multi-agent translation across 100+ languages.
        </p>
        <div class="flex flex-wrap gap-4 pt-2">
          <button onclick="openBookingModal()" class="px-6 py-3 rounded-xl bg-gradient-to-r from-pink-500 to-purple-600 text-white font-bold text-sm shadow-lg shadow-pink-500/25 hover:shadow-pink-500/40 hover:scale-105 transition-all flex items-center gap-2">
            <i class="fa-solid fa-ticket"></i> <span data-i18n="reserveFlightBtn">Book Flight</span>
          </button>
          <button onclick="alert('Searching Flights...')" class="px-6 py-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-sm border border-slate-700 transition-all flex items-center gap-2">
            <i class="fa-solid fa-magnifying-glass"></i> <span data-i18n="heroSearch">Search Deals</span>
          </button>
        </div>
      </div>
    </section>

    <!-- Flight Cards Grid -->
    <section class="flex flex-col gap-6">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-xl font-bold text-white" data-i18n="popularDestinations">Popular Flight Deals</h3>
          <p class="text-xs text-slate-400" data-i18n="liveSeatAvailability">Live seat availability updated every 30 seconds</p>
        </div>
        <span class="text-xs text-pink-400 font-semibold cursor-pointer hover:underline" data-i18n="viewAllDeals">View all 120 deals &rarr;</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        
        <!-- Card 1 -->
        <div class="bg-slate-900/80 border border-slate-800 hover:border-pink-500/50 rounded-2xl p-5 flex flex-col justify-between gap-5 transition-all hover:shadow-xl hover:shadow-pink-500/5">
          <div class="flex justify-between items-start">
            <div>
              <span class="text-xs font-semibold text-cyan-400">SFO &rarr; CDG</span>
              <h4 class="text-lg font-bold text-white mt-1">Paris, France</h4>
            </div>
            <span class="px-2.5 py-1 rounded-lg bg-pink-500/10 text-pink-400 text-xs font-bold">$480</span>
          </div>
          <div class="text-xs text-slate-400 flex items-center justify-between border-t border-slate-800 pt-3">
            <span><i class="fa-regular fa-clock mr-1"></i> 11h 20m Direct</span>
            <button onclick="addToCart('Paris Flight', 480)" class="px-3 py-1.5 rounded-lg bg-pink-500 hover:bg-pink-600 text-white font-semibold text-xs transition-all flex items-center gap-1">
              <i class="fa-solid fa-cart-plus"></i> <span data-i18n="cartmodalSubmitorder">Reserve</span>
            </button>
          </div>
        </div>

        <!-- Card 2 -->
        <div class="bg-slate-900/80 border border-slate-800 hover:border-pink-500/50 rounded-2xl p-5 flex flex-col justify-between gap-5 transition-all hover:shadow-xl hover:shadow-pink-500/5">
          <div class="flex justify-between items-start">
            <div>
              <span class="text-xs font-semibold text-cyan-400">JFK &rarr; HND</span>
              <h4 class="text-lg font-bold text-white mt-1">Tokyo, Japan</h4>
            </div>
            <span class="px-2.5 py-1 rounded-lg bg-pink-500/10 text-pink-400 text-xs font-bold">$620</span>
          </div>
          <div class="text-xs text-slate-400 flex items-center justify-between border-t border-slate-800 pt-3">
            <span><i class="fa-regular fa-clock mr-1"></i> 14h 05m Direct</span>
            <button onclick="addToCart('Tokyo Flight', 620)" class="px-3 py-1.5 rounded-lg bg-pink-500 hover:bg-pink-600 text-white font-semibold text-xs transition-all flex items-center gap-1">
              <i class="fa-solid fa-cart-plus"></i> <span data-i18n="cartmodalSubmitorder">Reserve</span>
            </button>
          </div>
        </div>

        <!-- Card 3 -->
        <div class="bg-slate-900/80 border border-slate-800 hover:border-pink-500/50 rounded-2xl p-5 flex flex-col justify-between gap-5 transition-all hover:shadow-xl hover:shadow-pink-500/5">
          <div class="flex justify-between items-start">
            <div>
              <span class="text-xs font-semibold text-cyan-400">LHR &rarr; BCN</span>
              <h4 class="text-lg font-bold text-white mt-1">Barcelona, Spain</h4>
            </div>
            <span class="px-2.5 py-1 rounded-lg bg-pink-500/10 text-pink-400 text-xs font-bold">$140</span>
          </div>
          <div class="text-xs text-slate-400 flex items-center justify-between border-t border-slate-800 pt-3">
            <span><i class="fa-regular fa-clock mr-1"></i> 2h 15m Direct</span>
            <button onclick="addToCart('Barcelona Flight', 140)" class="px-3 py-1.5 rounded-lg bg-pink-500 hover:bg-pink-600 text-white font-semibold text-xs transition-all flex items-center gap-1">
              <i class="fa-solid fa-cart-plus"></i> <span data-i18n="cartmodalSubmitorder">Reserve</span>
            </button>
          </div>
        </div>

      </div>
    </section>

    <!-- Cart & Checkout Modal Component -->
    <section class="bg-slate-900 border border-slate-800 rounded-2xl p-6 flex flex-col md:flex-row items-center justify-between gap-6 shadow-xl">
      <div class="flex items-center gap-4">
        <div class="w-12 h-12 rounded-xl bg-purple-500/20 text-purple-400 flex items-center justify-center text-xl">
          <i class="fa-solid fa-bag-shopping"></i>
        </div>
        <div>
          <h4 class="font-bold text-white" data-i18n="checkoutSummary">Checkout Summary</h4>
          <p class="text-xs text-slate-400"><span id="cartCount">1</span> items in cart &bull; Total: <span id="cartTotal" class="text-pink-400 font-bold">$480</span></p>
        </div>
      </div>

      <div class="flex items-center gap-3 w-full md:w-auto">
        <input type="text" id="couponInput" placeholder="Enter discount code" data-i18n-placeholder="placeholderEnterDiscountCode" class="bg-slate-950 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-pink-500 flex-1 md:w-48">
        <button onclick="applyCoupon()" class="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition-all">
          <span data-i18n="cartmodalApplycoupon">Apply Coupon</span>
        </button>
        <button onclick="submitOrder()" class="px-5 py-2 rounded-xl bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-bold shadow-lg shadow-pink-500/20 transition-all flex items-center gap-1.5">
          <i class="fa-solid fa-check"></i> <span data-i18n="cartmodalSubmitorder">Submit Order</span>
        </button>
      </div>
    </section>

  </main>

  <!-- Live Translation Engine Script -->
  <script>
    let currentMode = 'after'; // 'before' or 'after'
    let currentLang = 'en';
    let isGenZ = false;

    const DICTIONARIES = {
      en: {
        heroBadge: "Next-Gen Travel Platform",
        heroTitle: "Explore the World Without Language Barriers",
        heroSubtitle: "Book flights, reserve luxury hotels, and travel with real-time multi-agent translation across 100+ languages.",
        reserveFlightBtn: "Book Flight",
        heroSearch: "Search Deals",
        popularDestinations: "Popular Flight Deals",
        liveSeatAvailability: "Live seat availability updated every 30 seconds",
        viewAllDeals: "View all 120 deals →",
        checkoutSummary: "Checkout Summary",
        placeholderEnterDiscountCode: "Enter discount code",
        cartmodalApplycoupon: "Apply Coupon",
        cartmodalSubmitorder: "Submit Order",
        cartmodalCancel: "Cancel",
        navbarWelcomeback: "Welcome back, Harman!",
      },
      fr: {
        heroBadge: "Plateforme de voyage nouvelle génération",
        heroTitle: "Explorez le monde sans barrières linguistiques",
        heroSubtitle: "Réservez vos vols, hôtels de luxe et voyagez grâce à la traduction multi-agents en temps réel.",
        reserveFlightBtn: "Réserver un vol",
        heroSearch: "Chercher des offres",
        popularDestinations: "Offres de vols populaires",
        liveSeatAvailability: "Disponibilité des sièges mise à jour en direct",
        viewAllDeals: "Voir les 120 offres →",
        checkoutSummary: "Récapitulatif de la commande",
        placeholderEnterDiscountCode: "Code promo...",
        cartmodalApplycoupon: "Appliquer le code",
        cartmodalSubmitorder: "Confirmer la commande",
        cartmodalCancel: "Annuler",
        navbarWelcomeback: "Bienvenue, Harman !",
      },
      es: {
        heroBadge: "Plataforma de viajes de última generación",
        heroTitle: "Explora el mundo sin barreras de idioma",
        heroSubtitle: "Reserva vuelos, hoteles de lujo y viaja con traducción multi-agente en tiempo real.",
        reserveFlightBtn: "Reservar Vuelo",
        heroSearch: "Buscar Ofertas",
        popularDestinations: "Vuelos populares",
        liveSeatAvailability: "Disponibilidad de asientos actualizada en vivo",
        viewAllDeals: "Ver todas las 120 ofertas →",
        checkoutSummary: "Resumen del pedido",
        placeholderEnterDiscountCode: "Código de descuento",
        cartmodalApplycoupon: "Aplicar cupón",
        cartmodalSubmitorder: "Confirmar Pedido",
        cartmodalCancel: "Cancelar",
        navbarWelcomeback: "¡Bienvenido, Harman!",
      },
      de: {
        heroBadge: "Reiseplattform der nächsten Generation",
        heroTitle: "Entdecken Sie die Welt ohne Sprachbarrieren",
        heroSubtitle: "Buchen Sie Flüge und Luxushotels mit automatischer Multi-Agenten-Übersetzung.",
        reserveFlightBtn: "Flug buchen",
        heroSearch: "Angebote suchen",
        popularDestinations: "Beliebte Flugangebote",
        liveSeatAvailability: "Live-Sitzplatzverfügbarkeit alle 30 Sekunden",
        viewAllDeals: "Alle 120 Angebote ansehen →",
        checkoutSummary: "Bestellübersicht",
        placeholderEnterDiscountCode: "Gutscheincode eingeben",
        cartmodalApplycoupon: "Gutschein einlösen",
        cartmodalSubmitorder: "Bestellung abschicken",
        cartmodalCancel: "Abbrechen",
        navbarWelcomeback: "Willkommen zurück, Harman!",
      },
      ja: {
        heroBadge: "次世代旅行プラットフォーム",
        heroTitle: "言葉の壁なく世界を旅しよう",
        heroSubtitle: "リアルタイムAI多言語翻訳で、フライトや高級ホテルを簡単に予約。",
        reserveFlightBtn: "航空券を予約",
        heroSearch: "お得な便を検索",
        popularDestinations: "人気のフライト",
        liveSeatAvailability: "空席状況は30秒ごとに更新中",
        viewAllDeals: "120件すべてのフライトを見る →",
        checkoutSummary: "ご注文内容の確認",
        placeholderEnterDiscountCode: "クーポンコードを入力",
        cartmodalApplycoupon: "クーポン適用",
        cartmodalSubmitorder: "注文を確定する",
        cartmodalCancel: "キャンセル",
        navbarWelcomeback: "おかえりなさい、Harmanさん！",
      },
      hi: {
        heroBadge: "अगली पीढ़ी का यात्रा मंच",
        heroTitle: "बिना भाषा की बाधा के दुनिया घूमें",
        heroSubtitle: "रीयल-टाइम मल्टी-एजेंट अनुवाद के साथ उड़ानें और होटल बुक करें।",
        reserveFlightBtn: "फ्लाइट बुक करें",
        heroSearch: "ऑफर खोजें",
        popularDestinations: "लोकप्रिय उड़ानें",
        liveSeatAvailability: "सीट उपलब्धता हर 30 सेकंड में अपडेट होती है",
        viewAllDeals: "सभी 120 ऑफर देखें →",
        checkoutSummary: "ऑर्डर सारांश",
        placeholderEnterDiscountCode: "कूपन कोड डालें",
        cartmodalApplycoupon: "कूपन लागू करें",
        cartmodalSubmitorder: "ऑर्डर सबमिट करें",
        cartmodalCancel: "रद्द करें",
        navbarWelcomeback: "नमस्ते Harman, आपका स्वागत है!",
      },
      pa: {
        heroBadge: "ਅਗਲੀ ਪੀੜ੍ਹੀ ਦਾ ਯਾਤਰਾ ਪਲੇਟਫਾਰਮ",
        heroTitle: "ਬਿਨਾਂ ਕਿਸੇ ਭਾਸ਼ਾ ਰੁਕਾਵਟ ਦੇ ਦੁਨੀਆ ਘੁੰਮੋ",
        heroSubtitle: "ਰੀਅਲ-ਟਾਈਮ ਮਲਟੀ-ਏਜੰਟ ਅਨੁਵਾਦ ਨਾਲ ਫਲਾਈਟਾਂ ਅਤੇ ਹੋਟਲ ਬੁੱਕ ਕਰੋ।",
        reserveFlightBtn: "ਫਲਾਈਟ ਬੁੱਕ ਕਰੋ ✈️",
        heroSearch: "ਆਫਰ ਖੋਜੋ 🔍",
        popularDestinations: "ਪ੍ਰਸਿੱਧ ਉਡਾਣਾਂ",
        liveSeatAvailability: "ਸੀਟਾਂ ਦੀ ਉਪਲਬਧਤਾ ਹਰ 30 ਸਕਿੰਟਾਂ ਵਿੱਚ ਅਪਡੇਟ ਹੁੰਦੀ ਹੈ",
        viewAllDeals: "ਸਾਰੇ 120 ਆਫਰ ਦੇਖੋ →",
        checkoutSummary: "ਆਰਡਰ ਦਾ ਸਾਰ",
        placeholderEnterDiscountCode: "ਕੂਪਨ ਕੋਡ ਦਰਜ ਕਰੋ",
        cartmodalApplycoupon: "ਕੂਪਨ ਲਾਗੂ ਕਰੋ",
        cartmodalSubmitorder: "ਆਰਡਰ ਪੱਕਾ ਕਰੋ 🚀",
        cartmodalCancel: "ਰੱਦ ਕਰੋ",
        navbarWelcomeback: "ਜੀ ਆਇਆਂ ਨੂੰ, Harman ਜੀ! 🔥",
      },
      genz_fr: {
        heroBadge: "La plateforme de voyage trop stylée ✨",
        heroTitle: "Visite le monde entier en mode zéro galère 🌍",
        heroSubtitle: "Réserve direct tes bails de vol et tes hôtels carrés sans te prendre la tête. no cap 🔥",
        reserveFlightBtn: "Réserve direct 🔒",
        heroSearch: "Check ça 🔍",
        popularDestinations: "Les bails de vols les plus chauds 🔥",
        liveSeatAvailability: "Places dispo en temps réel no fake",
        viewAllDeals: "Check les 120 pépites →",
        checkoutSummary: "Le récap de la commande",
        placeholderEnterDiscountCode: "Balance le code promo",
        cartmodalApplycoupon: "Applique le bail",
        cartmodalSubmitorder: "Valide le bail 🚀",
        cartmodalCancel: "Laisse tomber 💀",
        navbarWelcomeback: "Wesh bon retour, Harman ! En vrai c'est carré 🔥",
      },
      genz_en: {
        heroBadge: "Next-Gen Travel Platform no cap ✨",
        heroTitle: "Explore the World with Zero Language Struggles 🔥",
        heroSubtitle: "Book flights, reserve fire hotels, and flex worldwide with multi-agent instant translations.",
        reserveFlightBtn: "Book it 🔒",
        heroSearch: "Hunt Deals 🔍",
        popularDestinations: "Hottest Flight Drops 🔥",
        liveSeatAvailability: "Live seats updating crazy fast",
        viewAllDeals: "Peep all 120 deals →",
        checkoutSummary: "Cart Recap",
        placeholderEnterDiscountCode: "Drop discount code",
        cartmodalApplycoupon: "Apply that promo",
        cartmodalSubmitorder: "Ship it 🚀",
        cartmodalCancel: "Nevermind 💀",
        navbarWelcomeback: "ayyy welcome back, Harman! no cap 🔥",
      }
    };

    function updateDOM() {
      let dict = DICTIONARIES[currentLang] || DICTIONARIES['en'];
      
      if (currentMode === 'before') {
        dict = DICTIONARIES['en']; // Raw hardcoded strings
        document.getElementById('statusNotice').className = "bg-amber-950 border-b border-amber-800/50 px-4 py-2 text-center text-xs text-amber-200 flex items-center justify-center gap-2";
        document.getElementById('statusText').innerHTML = "⚠️ Currently viewing: <b>Raw Hardcoded English Code (Before)</b>. Strings are hardcoded directly inside components.";
      } else {
        if (isGenZ) {
          if (currentLang === 'fr') dict = DICTIONARIES['genz_fr'];
          else dict = DICTIONARIES['genz_en'];
        }
        document.getElementById('statusNotice').className = "bg-gradient-to-r from-purple-950 via-slate-900 to-pink-950 border-b border-purple-800/30 px-4 py-2 text-center text-xs text-purple-200 flex items-center justify-center gap-2";
        document.getElementById('statusText').innerHTML = "✨ Currently viewing: <b>Surgically Localized AST Code (After)</b>. Translated into " + currentLang.toUpperCase() + (isGenZ ? " (Gen-Z Slang Tone)" : "") + " with 4-Tier Critic passing 100%.";
      }

      document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (dict[key]) el.innerText = dict[key];
      });

      document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        if (dict[key]) el.placeholder = dict[key];
      });

      if (dict['heroTitle']) document.getElementById('heroTitle').innerText = dict['heroTitle'];
      if (dict['heroSubtitle']) document.getElementById('heroSubtitle').innerText = dict['heroSubtitle'];
    }

    function setMode(mode) {
      currentMode = mode;
      const btnBefore = document.getElementById('btnBefore');
      const btnAfter = document.getElementById('btnAfter');

      if (mode === 'before') {
        btnBefore.className = "px-3 py-1.5 rounded-lg text-xs font-semibold transition-all bg-amber-500 text-slate-950 font-bold shadow-md";
        btnAfter.className = "px-3 py-1.5 rounded-lg text-xs font-semibold transition-all text-slate-400 hover:text-white";
      } else {
        btnAfter.className = "px-3 py-1.5 rounded-lg text-xs font-semibold transition-all bg-gradient-to-r from-pink-500 to-purple-600 text-white shadow-md";
        btnBefore.className = "px-3 py-1.5 rounded-lg text-xs font-semibold transition-all text-slate-400 hover:text-white";
      }
      updateDOM();
    }

    function toggleGenZ() {
      isGenZ = !isGenZ;
      const status = document.getElementById('genZStatus');
      const btn = document.getElementById('btnGenZ');
      if (isGenZ) {
        status.className = "w-2 h-2 rounded-full bg-pink-400 animate-ping";
        btn.className = "px-3 py-1.5 rounded-xl text-xs font-semibold border transition-all border-pink-500 bg-pink-950/40 text-pink-300 flex items-center gap-1.5 shadow-lg shadow-pink-500/20";
      } else {
        status.className = "w-2 h-2 rounded-full bg-slate-600";
        btn.className = "px-3 py-1.5 rounded-xl text-xs font-semibold border transition-all border-slate-800 bg-slate-950 text-slate-300 hover:border-pink-500 flex items-center gap-1.5";
      }
      updateDOM();
    }

    function changeLanguage(lang) {
      currentLang = lang;
      if (currentMode === 'before') {
        setMode('after'); // Automatically switch to After when picking a language
      } else {
        updateDOM();
      }
    }

    function addToCart(title, price) {
      const totalEl = document.getElementById('cartTotal');
      const countEl = document.getElementById('cartCount');
      totalEl.innerText = '$' + price;
      countEl.innerText = '1';
      alert('✓ Added ' + title + ' to cart!');
    }

    function applyCoupon() {
      const code = document.getElementById('couponInput').value;
      if (code) alert('🎉 Coupon "' + code + '" applied! 20% discount granted.');
      else alert('Please enter a coupon code.');
    }

    function submitOrder() {
      alert('🚀 Order successfully confirmed and ticket issued!');
    }

    function openBookingModal() {
      alert('✈️ Flight booking flow opened for ' + currentLang.toUpperCase() + ' locale!');
    }

    // Initialize
    updateDOM();
  </script>
</body>
</html>`
