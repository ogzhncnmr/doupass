# doupass — Ajan Politikası Motoru + Taşınabilir Politika Formatı

Durum: S0 tamamlandı; adversarial review bulguları entegre
Tarih: 2026-09-12
Hedef: Ajanların araç çağrılarını yerel olarak denetleyen, taşınabilir açık politika formatı (agents için Sigma) + local-first tek-binary runtime politika motoru.

---

## 1. Konumlandırma (netleştirilmiş)

**Tek cümle:** "Aynı politika dosyasıyla farklı ajanları denetle: MCP araçları için proxy, harness'ın yerleşik araçları için hook adaptörü."

**Farklılaşma:**
1. **Tek taşınabilir politika** — harness'ların kendi izin sistemleri (Claude Code allow/deny, Codex approval modları) birbirinden kopuk ve taşınamaz. doupass "bir politika, tüm ajanlar" vaadinin sahibi.
2. **Local-first, bireysel geliştirici UX'i** — mevcut ciddi araçlar K8s/kurumsal (toolhive, agyn, ThinkWatch, lunar) veya nokta-tarama (agentseal, MCP-Defender).
3. **Açık politika formatı + referans motor ayrımı** — format topluluğa, motor bize. Agent-Threat-Rule deseni tespit kuralları için bunu yaptı; izin politikası için kimse yapmadı.

**Dürüst sınır (v0.1):** Denetim iki yüzeyde çalışır: (a) MCP tool çağrıları (proxy), (b) Claude Code'un yerleşik araçları (PreToolUse hook). Diğer harness'ların yerleşik araçları v0.1'de kapsam dışı — README'de açıkça yazılır. "Agent-agnostik" iddiası v0.2'de hook'lar genişledikçe tamamlanır.

**Kaçınılacak savaş:** Kurumsal MCP gateway pazarı (IBM mcp-context-forge, agentgateway, toolhive). Oraya ancak v1 sonrası, merkezi politika yönetimi ürünüyle girilir.

**En zayıf varsayım ve doğrulaması:** "Taşınabilir politika formatı benimsenir" varsayımı. Doğrulama: S7.5'te 5 harici geliştiriciyle format walkthrough'u; kabul edilmezse konumlandırma "Claude Code için güvenlik katmanı"na daraltılır.

---

## 2. Rakip taraması özeti (Eylül 2026, GitHub)

| Proje | Yıldız | Dil | Kategori | doupass'a karşı duruş |
|---|---|---|---|---|
| Claude Code izin sistemi + sandbox | — | — | Harness-native izin | **Asıl rakip**; taşınamaz, harness'a bağlı |
| Codex approval/sandbox modları | — | — | Harness-native izin | **Asıl rakip**; taşınamaz |
| agentgateway/agentgateway | 4.8k | Rust | Agentic proxy | K8s/enterprise |
| IBM/mcp-context-forge | 4.5k | Python | Gateway/registry/proxy | Kurumsal, ağır |
| nolabs-ai/nono | 4.1k | Rust | Sandbox execution | Politika dili yok |
| stacklok/toolhive | 2.2k | Go | MCP server runner | K8s odaklı |
| luckyPipewrench/pipelock | 841 | Go | Agent firewall + egress | En yakın rakip; taşınabilir politika standardı yok |
| zerobox | 716 | Rust | Process sandbox | Politika katmanı yok |
| emilia-protocol | 649 | TS | Yetki kontrol düzlemi | Ağır formal protokol |
| hol-guard | 599 | Python | Agent antivirüsü | Tarama ağırlıklı |
| Adrian | 560 | Python | Runtime izleme | Deterministik engelleme zayıf |
| Agent-Threat-Rule | 389 | TS | Tespit kural standardı | İzin/politika değil, tespit |
| agentseal | 371 | Python | Tarama toolkit | Nokta tarama |
| MCP-Defender | 257 | TS | Desktop tarayıcı/engelleyici | Runtime politika yok |
| node9-proxy | 211 | TS | "Sudo" governance | Dar kapsam |

**Not:** Tablodaki tüm sayılar lansman öncesi (S8) yeniden doğrulanır; tarih damgası eklenir.

**Sonuç:** Alan kalabalık ama (a) hiçbiri "varsayılan" değil, (b) taşınabilir ortak politika formatı yok, (c) bireysel geliştirici için düşük sürtünmeli seçenek yok, (d) harness-native izinler taşınamıyor. Fırsat: taşınabilirlik + UX.

---

## 3. Hedef kullanıcı ve kullanım senaryoları

- **Birincil:** Claude Code/Codex/OpenCode/Cursor kullanan bireysel geliştiriciler ve küçük ekipler.
- **İkincil:** Şirket içi platform ekipleri (v0.2+ merkezi politika ile).
- **Üçüncül:** Kural/politika katkıcıları.

**v0.1'de gerçekten çalışan senaryolar:**
1. "MCP filesystem sunucusu ~/.ssh okumasın" → proxy deny. (MCP yüzeyi)
2. "npm install gibi Bash komutları benden onay istesin" → Claude Code hook ask. (hook yüzeyi)
3. "Bilinen kötü MCP sunucusuna bağlanma" → server bazlı deny + unknown_server. (MCP yüzeyi)
4. "Bugün hangi tehlikeli çağrı yapıldı?" → audit log. (her iki yüzey)

**v0.1'de çalışmayan (README'de non-goal):**
- Diğer harness'ların yerleşik araç denetimi (Codex/OpenCode/Cursor native araçlar).
- Oturumlar arası veri akışı korelasyonu (taint tracking): izinli bir okuma, izinli bir curl ile sızdırılabilir; v0.1 bunu yakalamaz.

---

## 4. Mimari (v0.1)

```
Claude Code ──(PreToolUse hook)──> doupass hook ──┐
                                              |
Claude Code / OpenCode ──(MCP stdio)──> doupass proxy (sunucu başına 1 süreç)
                                              |
                        +---------------------v--------------------+
                        |              doupass (tek Go binary)         |
                        |                                          |
                        |  [Policy engine]                         |
                        |    parse (YAML -> JSON Schema)           |
                        |    match (kanonik path + glob/regex)     |
                        |    decide: allow | deny | ask | log      |
                        |                                          |
                        |  [ask kanalı]                            |
                        |    /dev/tty | CONIN$; yoksa fail-closed  |
                        |    (ask_fallback: deny, ask_timeout)     |
                        |                                          |
                        |  [Audit logger]                          |
                        |    JSONL + SHA-256 hash chain, dosya     |
                        |    kilidi, 0600; tamper-evident          |
                        +------------------------------------------+
                                              |
                        Upstream MCP sunucusu (tek; agregasyon v0.2)
```

**Tasarım kararları:**
- **Sunucu başına bir proxy süreci** (`doupass proxy -- <server cmd>`): request-ID yeniden eşleme, tool adı çakışması, capability birleştirme gibi multiplexer riskleri v0.1'e girmez. Çoklu sunucu agregasyonu v0.2.
- **stdio framing:** satır-delimli JSON-RPC; okuma tamponu açık üst sınırla (varsayılan 10 MB) ve büyük-payload testiyle doğrulanır.
- **Path eşleştirme kanonikleştirme:** `filepath.Clean` + `EvalSymlinks` + Windows'ta case-fold + `\` normalizasyonu; `${WORKSPACE}` sembolik link çözümü sonrası eşleşir. Aksi halde `**/.ssh/**` kuralı kolayca atlatılır.
- **ask transport:** `/dev/tty` (Unix) / `CONIN$` (Windows) üzerinden sorar; TTY yoksa (GUI harness, daemon) `ask_fallback: deny` (varsayılan, fail-closed) ve `ask_timeout` uygulanır. Hook yüzeyinde ise soru harness'a bırakılır (izin kararını harness gösterir).
- **Redaction v0.1 dışı** (semantiği olgun değil; v0.2).
- **Proxy'nin kendi yüzeyi:** downstream sunucu timeout'u, çıktı boyut sınırı, bozuk mesaj karantinası — S3 threat model'de.

**Politika formatı (taslak v0.1):**

```yaml
version: "0.1"
name: default-dev
rules:
  - id: block-ssh-credentials
    match:
      tool: "filesystem.*"
      args:
        path: "**/.ssh/**"
    action: deny
    reason: "SSH anahtarlarına erişim engellendi"
  - id: ask-shell-install
    match:
      surface: hook            # hook | mcp | any
      tool: "Bash"
      args:
        command: "*npm install*"
    action: ask
  - id: allow-workspace-read
    match:
      tool: "filesystem.read"
      args:
        path: "${WORKSPACE}/**"
    action: allow
defaults:
  action: ask
  unknown_server: deny
  ask_fallback: deny
  ask_timeout_seconds: 60
audit:
  path: "~/.doupass/audit.jsonl"
  hash_chain: true
```

**Eşleştirme semantiği:** glob + regex; kurallar sırayla değerlendirilir, ilk eşleşen kazanır; aynı özgüllükte `deny > ask > allow`. (Spec'te tam olarak tanımlanır.)

**Tehdit modeli (özet):**
- Prompt injection sonrası MCP exfiltration → proxy deny + audit.
- Prompt injection sonrası Bash ile kimlik okuma/yıkım → hook deny/ask + audit.
- Kötü niyetli MCP sunucusu → unknown_server deny; timeout/boyut sınırları.
- Tool poisoning → v0.2 (temel tarama yok v0.1'de).
- **Non-goals:** OS sandbox kaçışı, malware tespiti, model hizalaması, oturumlar arası taint analizi.

---

## 5. MVP kapsamı (v0.1)

**İçinde:**
- Tek binary `doupass` (Go), Windows/macOS/Linux + goreleaser ile checksum/imza'lı sürüm.
- MCP stdio proxy (sunucu başına süreç) + Claude Code PreToolUse hook adaptörü, aynı politika motoru.
- Politika motoru: allow/deny/ask/log; glob + regex; kanonik path eşleştirme.
- `doupass init`, `doupass proxy`, `doupass hook`, `doupass policy test`, `doupass log tail|verify`.
- 15+ hazır kural, 3 örnek politika (default-dev, red-team, read-only).
- Audit log: JSONL + hash chain + dosya kilidi + `doupass log verify`.
- Dokümantasyon (repo içi): quickstart (5 dk), politika referansı, threat model, dürüst rakip karşılaştırması.
- Test: core coverage ≥ %80; mock MCP sunucusu + scripted hook transcript e2e.

**Dışında (v0.2+):**
- GUI, bulut paneli, takım yönetimi, SSO.
- HTTP egress proxy, OS sandbox, redaction, `doupass scan` (tool poisoning taraması).
- Codex/OpenCode/Cursor hook adaptörleri (OpenCode MCP wiring v0.1'e dahil; hook'lar v0.2).
- Çoklu MCP sunucu agregasyonu, GitHub Pages sitesi, ML injection tespiti.

**Başarı kriterleri (lansmandan 4 hafta sonra):**
- Show HN; ilk hafta 150+ yıldız.
- 10 harici kullanıcının issue/politika katkısı.
- govulncheck + gosec temiz (tarihli kanıt CI'da).
- S7.5 format doğrulaması: 5 geliştiriciden ≥3'ü formatı anlar ve kendi kuralını yazar.

---

## 6. Teknoloji kararları

| Karar | Seçim | Gerekçe |
|---|---|---|
| Dil | **Go** | Tek binary, kolay cross-compile, hızlı iterasyon, MCP ekosistemi |
| Politika | YAML + JSON Schema | İnsan-okur, IDE doğrulaması, kolay diff |
| Audit | JSONL + SHA-256 hash chain | Basit, taşınabilir, tamper-evident |
| CLI | stdlib + cobra | Standart, hafif |
| Sürüm | goreleaser + minisign/sigstore | Matrix build, checksum, imza |
| CI | GitHub Actions (build/test/lint/govulncheck/windows runner) | Standart |
| Test | Go test + mock MCP + scripted hook | Deterministik e2e |

Rust alternatifi: güçlü güvenlik kimliği ama MVP hızı düşük; ileride performans-kritik parçalar taşınabilir.

---

## 7. Yol haritası — Adımlar (her adım 1 PR)

Bağımlılık grafiği: S0 → S1 → S2 → S3 → S4 → S5 → S6 → S7; S7 → S7.5 → S8 → S8.5 → S9.
(S3 ve S5 farklı yüzeyler; S5, S3 bitmeden başlayabilir ama ask kanalı S3'te tanımlanır.)

Tahmini efor (part-time, haftada ~10 saat): S0–S2: 2 hafta; S3–S4: 3 hafta; S5–S6: 2 hafta; S7–S8: 2 hafta; S8.5–S9: 1 hafta. **Toplam ~10-12 hafta.**

### S0 — Repo bootstrap ve isim doğrulama (3-4 gün)
- **Bağlam:** Boş dizin; git + gh hazır; proje isimsiz.
- **İşler:** İsim adaylarını GitHub/npm'de kontrol et (doupass — npm/GitHub/site temiz). `go mod init`, `git init`, Apache-2.0 LICENSE, README iskeleti, **AGENTS.md (İngilizce)**, CI workflow (build/test), .gitignore, `docs/threat-model.md` taslağı.
- **Doğrulama:** `go build ./...` ve CI yeşil (boş `main.go` ile).
- **Çıkış kriteri:** GitHub'a push edildi; isim kesinleşti.

### S1 — Politika formatı v0 spesifikasyonu (3-4 gün)
- **Bağlam:** Format tüm projenin çekirdeği; erken dondurulur, geriye uyumlu evrilir. Spec lisansı: **CC0** (S1'de karar).
- **İşler:** `spec/policy-v0.md` + `spec/policy-v0.schema.json`; eşleştirme semantiği (glob/regex, öncelik, ilk-eşleşen); `${WORKSPACE}`/`${HOME}`; `surface` alanı; 10 örnek kural; test fixture'ları (pozitif/negatif).
- **Doğrulama:** `check-jsonschema --schemafile spec/policy-v0.schema.json fixtures/*.yaml` tümünü doğrular; 3 sınır durumu (iç içe glob, regex kaçışı, öncelik) dokümante ve fixture'lanmış.
- **Çıkış kriteri:** Spec merge; tüm fixture'lar şemadan geçer.

### S2 — Çekirdek politika motoru (1 hafta)
- **Bağlam:** S1 spec'i hazır; saf kütüphane, MCP yok.
- **İşler:** `internal/policy`: parse, validate, match (kanonik path: Clean+EvalSymlinks+case-fold, `\` normalizasyonu), decide (deny>ask>allow). Windows/macOS path fixture'ları. Tablo-tabanlı testler + benchmark.
- **Doğrulama:** `go test ./internal/policy/... -cover` ≥ %80; symlink ve Windows path atlatma testleri var.
- **Çıkış kriteri:** Motor bağımsız kullanılabilir; API dokümante.

### S3 — MCP stdio proxy (1.5 hafta)
- **Bağlam:** S2 motoru hazır; ilk denetim yüzeyi.
- **İşler:** `doupass proxy -- <server cmd>`: tek downstream sunucu; satır-delimli JSON-RPC; 10 MB üst sınır + aşımda deny; downstream timeout; PATHEXT çözümü (Windows `npx.cmd`), Job Object ile süreç ağacı sonlandırma; ask kanalı (`/dev/tty`/`CONIN$`, fail-closed). Windows CI runner.
- **Doğrulama:** Mock MCP sunucusuyla e2e: allow geçer, deny engellenir + ajana anlamlı hata, ask onay/red akışı; büyük payload testi; süreç ölümü testi.
- **Çıkış kriteri:** Gerçek filesystem MCP sunucusuyla manuel smoke test + scripted transcript.

### S4 — CLI + audit log (1.5 hafta)
- **Bağlam:** Proxy çalışıyor; kullanılabilirlik ve kanıt katmanı.
- **İşler:** `doupass init`, `doupass policy test`, `doupass log tail|verify`; audit entry şeması dondurulur (canonical JSON); genesis kuralı; os-file-lock ile çoklu süreç ekleme; dosya izinleri 0600/0700; argümanlar audit'te maskelenir; "tamper-evident, tamper-proof değil" dokümante.
- **Doğrulama:** E2E: init → test → proxy → verify. Kurcalama (tek satır değiştirme) `doupass log verify` ile tespit edilir. Eşzamanlı 2 proxy yazımı testi.
- **Çıkış kriteri:** Quickstart bu komutlarla 5 dakikada çalışır.

### S5 — Claude Code hook adaptörü (1 hafta)
- **Bağlam:** İkinci denetim yüzeyi: harness'ın yerleşik araçları (Bash/Read/Write/Edit).
- **İşler:** `doupass hook claude` (PreToolUse): JSON girdi → politika → allow/deny/ask çıktısı; ask kararını harness'a bırakma biçimi; `doupass install claude` kurulum komutu (settings.json'a güvenli ekleme, yedek, manifest) + `doupass uninstall claude`.
- **Doğrulama:** Scripted transcript testi (hook girdi/çıktı fixture'ları); gerçek Claude Code oturumunda manuel deny/ask demosu (çıkış kanıtı olarak asciinema).
- **Çıkış kriteri:** Hook e2e yeşil; demo README'de.

### S6 — Hazır kural seti v1 (3-4 gün)
- **Bağlam:** S2 + S4 hazır (`doupass policy test` ile doğrulanır).
- **İşler:** `rules/`: kimlik dosyaları (~/.ssh, ~/.aws, .env, kubeconfig), yıkıcı komutlar (rm -rf /, force push), exfil vektörleri (curl -d @, base64 pipe), paket kurulumları (postinstall), veritabanı (DROP). Her kural için pozitif/negatif fixture.
- **Doğrulama:** `doupass policy test rules/*.yaml` tüm senaryoları geçer.
- **Çıkış kriteri:** 3 örnek politika + 15 kural repoda.

### S7 — OpenCode MCP wiring + manuel kurulum dokümanı (3-4 gün)
- **Bağlam:** Claude Code dışında ilk genişleme; hook değil, yalnızca MCP yüzeyi.
- **İşler:** `doupass install opencode` (MCP config dönüşümü, manifest, uninstall); Codex/Cursor için manuel wiring dokümanı.
- **Doğrulama:** OpenCode'da MCP deny e2e; fixture config ile install/uninstall testleri (idempotent).
- **Çıkış kriteri:** 2 harness kurulumlu destek; README matrisi dürüst ("native tool denetimi: Claude Code ✅, diğerleri v0.2").

### S7.5 — Format doğrulama (3-5 gün, S7 ile kısmen paralel)
- **Bağlam:** En zayıf varsayım: format benimsenir mi?
- **İşler:** 5 harici geliştiriciye format walkthrough'u; geri bildirimle spec revizyonu; anlama/yazma metrikleri.
- **Doğrulama:** ≥3/5 kendi kuralını yazabildi.
- **Çıkış kriteri:** Spec v0.1 donduruldu veya konumlandırma daraltıldı (risk planına göre).

### S8 — Dokümantasyon ve rakip tablosu (3-4 gün)
- **Bağlam:** Özellikler donduruldu.
- **İşler:** README (tek sayfa kavratma), quickstart, politika referansı, threat model, FAQ; rakip tablosu **yeniden doğrulanmış sayılarla + tarih damgası**.
- **Doğrulama:** Kullanıcı testi: yeni biri dokümandan 5 dk'da kurar.
- **Çıkış kriteri:** Docs tamam; README hazır.

### S8.5 — Sürüm mühendisliği (2-3 gün)
- **Bağlam:** MVP üç platformda kurulabilir olmalı.
- **İşler:** goreleaser (matrix build, checksum, minisign/sigstore), `v0.1.0-rc1` tag, kurulum talimatları, `go install` yolu.
- **Doğrulama:** Temiz makinede üç platformdan indirilen binary `doupass version` çalıştırır; checksum doğrulanır.
- **Çıkış kriteri:** RC binary'leri yayında.

### S9 — Lansman (2-3 gün)
- **Bağlam:** S7 + S8 + S8.5 tamam (bağımlılık düzeltildi).
- **İşler:** Show HN + 60 sn demo; r/ClaudeAI, r/LocalLLaMA, r/mcp; awesome-mcp-security PR; MCP dizini kaydı; dev.to TR/EN tanıtım yazısı.
- **Doğrulama:** İlk 72 saat metrik takibi (yıldız, indirme, issue).
- **Çıkış kriteri:** v0.1.0 yayında; geri bildirim döngüsü başladı.

### S10 — v0.2 planlaması (lansman sonrası)
Hook'ların Codex/OpenCode/Cursor'a genişletilmesi, redaction, `doupass scan` (tool poisoning), egress kuralları, taint tracking araştırması, kural registry'si, merkezi politika (open-core) araştırması.

---

## 8. Lisans ve sürdürülebilirlik

- **Kod:** Apache-2.0 (şirket içi benimseme engelini azaltır).
- **Spec:** CC0 (S1'de kesinleşir; katkı koşulları net olsun diye erken karar).
- **Model:** Open-core: yerel motor ücretsiz kalır; merkezi politika + SSO + raporlama ücretli (v1+). Şimdilik GitHub Sponsors + danışmanlık.
- **Topluluk:** good-first-issue etiketleri; kural katkı şablonu; aylık sürüm ritmi; AGENTS.md/spec/issues İngilizce, plan/iletişim Türkçe olabilir.

---

## 9. Riskler

| Risk | Olasılık | Etki | Azaltma |
|---|---|---|---|
| Taşınabilirlik vaadi benimsenmez (format tutmaz) | Orta | Yüksek | S7.5 doğrulaması; gerekirse "Claude Code güvenlik katmanı"na daralt |
| Harness-native izinler yeterince iyi (özellikle Claude Code) | Yüksek | Orta | Fark: taşınabilirlik + MCP yüzeyi + audit birleşikliği |
| agentgateway/IBM bireysel moda girer | Orta | Yüksek | Hız; spec'i topluluk standardı yapma hamlesi |
| `ask` TTY sorunu (GUI harness) | Yüksek | Orta | Fail-closed varsayılan; hook yüzeyinde soruyu harness'a bırakma |
| Prompt-injection tespitinde yanlış pozitif beklentisi | Orta | Orta | v0.1 deterministik; non-goal'da net yaz |
| Tek geliştirici tükenmesi | Orta | Yüksek | Kapsam disiplini (redaction/scan/site v0.2); topluluk devri |
| "Kural motoru atlatılır" eleştirisi | Orta | Orta | Threat model'de sınır + non-goals; OS sandbox v0.2 roadmap'te |
| Windows süreç/PTY sürprizleri | Orta | Orta | Windows CI, PATHEXT/Job Object işleri S3'te |

---

## 10. Anti-pattern kataloğu (bu proje için)

1. **MCP-only'u agent-agnostik sanmak** — hook yüzeyi olmadan yerleşik araçlar (Bash/Read) denetlenmez; doküman ve iddialar buna göre dürüst olmalı.
2. **Kurumsal gateway yarışına erken girmek.**
3. **UI'yı erken yapmak** — CLI + dosya tabanlı politika yeterli.
4. **Spec dondurmadan kod yığmak** — S1 çıktısı olmadan S2 başlamaz.
5. **Tespit kurallarını izin kurallarıyla karıştırmak.**
6. **Lansmanı ertelemek** — S8.5 sonrası hemen S9.
7. **Redaction/scan gibi olgunlaşmamış özellikleri MVP'ye sıkıştırmak** — v0.2.

---

## 11. Plan mutasyon protokolü

- Adım bölme: yeni adım Sx.1 olarak eklenir, bağımlılık kenarı güncellenir.
- Atlama: gerekçe "Atlanan adımlar" bölümüne yazılır.
- Sıra değişimi: bağımlılık grafiği güncellenmeden uygulanmaz.
- İptal: kapsam dışına taşınır; issue açılır.

---

## 12. Açık sorular

1. Proje adı — doupass (S0'da kesinleşti).
2. Binary adı — `doupass` (tek binary).
3. Kural eşleştirme: glob yeterli mi, ifade dili (CEL) gerekli mi? (v0.2 kararı)
4. İkinci harness: OpenCode varsayıldı; Codex mi olsun?
5. Hook yüzeyi genişletmesi hangi harness ile başlar? (v0.2: Codex)
