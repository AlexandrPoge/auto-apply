package main

import (
	"bufio"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type letterRequest struct {
	CandidateProfile string `json:"candidate_profile"`
	VacancyText      string `json:"vacancy_text"`
	VacancyTitle     string `json:"vacancy_title"`
}
type letterResponse struct {
	Letter string `json:"letter"`
	Source string `json:"source"`
}

func main() {
	loadDotEnv(".env")
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/letter", handleLetter)
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Reply Studio is running at http://localhost%s", addr)
	log.Fatal(server.ListenAndServe())
}

func handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, nil); err != nil {
		log.Printf("render home: %v", err)
	}
}
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"ok"}`)
}
func handleLetter(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "используйте POST")
		return
	}
	var input letterRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "неверный JSON: "+err.Error())
		return
	}
	input.CandidateProfile = strings.TrimSpace(input.CandidateProfile)
	input.VacancyText = strings.TrimSpace(input.VacancyText)
	input.VacancyTitle = strings.TrimSpace(input.VacancyTitle)
	if input.CandidateProfile == "" {
		writeError(w, http.StatusBadRequest, "добавьте краткую информацию о себе")
		return
	}
	if input.VacancyText == "" && input.VacancyTitle == "" {
		writeError(w, http.StatusBadRequest, "добавьте ссылку, название или текст вакансии")
		return
	}
	writeJSON(w, http.StatusOK, letterResponse{Letter: buildTemplateLetter(input), Source: "local_template"})
}
func buildTemplateLetter(input letterRequest) string {
	title := strings.TrimSpace(input.VacancyTitle)
	if title == "" {
		title = firstLine(input.VacancyText, 120)
	}
	if title == "" || title == "Вакансия на hh.ru" {
		title = "вакансия в вашей команде"
	}
	summary := firstSentences(input.CandidateProfile, 2, 520)
	return "Здравствуйте!\n\nЗаинтересовала вакансия «" + title + "».\n\nКоротко о релевантном опыте: " + summary + "\n\nБуду рад подробнее обсудить, как смогу быть полезен команде. Спасибо за рассмотрение!"
}
func firstLine(text string, limit int) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "Вакансия:"))
		if line != "" {
			return truncateText(line, limit)
		}
	}
	return ""
}
func firstSentences(text string, amount, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	end, found := 0, 0
	for index, char := range text {
		if char == '.' || char == '!' || char == '?' {
			end, found = index+len(string(char)), found+1
			if found == amount {
				break
			}
		}
	}
	if end > 0 {
		text = text[:end]
	}
	return truncateText(text, limit)
}
func truncateText(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}

// loadDotEnv lets a local .env work without requiring the user to export it manually.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, strings.Trim(strings.TrimSpace(value), "\"'"))
	}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

var pageTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Reply Studio — сопроводительные письма</title><style>
:root{color-scheme:dark;font-family:Inter,ui-sans-serif,system-ui,-apple-system,sans-serif;color:#f7f8ff;background:#0a0d1a}*{box-sizing:border-box}body{margin:0;min-width:320px;background:radial-gradient(circle at 18% -10%,#30408f 0,transparent 32rem),radial-gradient(circle at 100% 18%,#562d77 0,transparent 30rem),#0a0d1a}.shell{width:min(1180px,100%);margin:auto;padding:26px 22px 68px}.nav{display:flex;align-items:center;justify-content:space-between;gap:20px}.brand{display:flex;align-items:center;gap:10px;font-weight:750;letter-spacing:-.035em}.mark{display:grid;place-items:center;width:32px;height:32px;border-radius:10px;background:linear-gradient(135deg,#9b87ff,#4de2cb);color:#101327;font-size:18px}.state{border:1px solid #455178;border-radius:999px;padding:7px 11px;color:#bdc9e9;font-size:12px}.hero{padding:72px 0 34px;max-width:810px}.eyebrow{margin:0 0 12px;color:#9af2e5;font-size:12px;font-weight:760;letter-spacing:.14em;text-transform:uppercase}.hero h1{max-width:760px;margin:0;font-size:clamp(39px,7vw,74px);line-height:.98;letter-spacing:-.065em}.hero h1 span{color:#9e8bff}.lead{margin:23px 0 0;max-width:680px;color:#b7c1dc;font-size:17px;line-height:1.65}.steps{display:flex;flex-wrap:wrap;gap:9px;margin-top:28px}.step{border:1px solid #313b5d;border-radius:999px;padding:8px 12px;color:#c6cfea;font-size:13px}.step b{margin-right:5px;color:#8ef2df}.workbench{display:grid;grid-template-columns:minmax(0,1.05fr) minmax(330px,.95fr);gap:16px;align-items:start}.card{position:relative;overflow:hidden;border:1px solid #2d3758;border-radius:22px;background:linear-gradient(145deg,#171c31e8,#0e1222ed);box-shadow:0 24px 70px #0000002e;padding:22px}.card:before{position:absolute;inset:0;pointer-events:none;content:"";background:linear-gradient(120deg,#fff1 0,transparent 24%)}.card>*{position:relative}.section-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;margin-bottom:20px}.index{color:#9e8bff;font-size:12px;font-weight:800;letter-spacing:.12em}.card h2{margin:5px 0 0;font-size:22px;letter-spacing:-.035em}.subtle{margin:7px 0 0;color:#aab4d0;font-size:13px;line-height:1.55}.field{display:block;margin-top:17px;color:#e9edfa;font-size:13px;font-weight:680}.field span{display:flex;justify-content:space-between;gap:12px;margin-bottom:8px}.field small{color:#95a1bf;font-weight:500}textarea,input{width:100%;border:1px solid #3a466b;border-radius:13px;background:#0a0e1d;color:#f5f7ff;font:inherit;outline:0;transition:border .18s,box-shadow .18s,background .18s}textarea{min-height:205px;padding:14px;resize:vertical;line-height:1.55}input{padding:13px}textarea:focus,input:focus{border-color:#9b87ff;background:#0d1224;box-shadow:0 0 0 4px #9b87ff24}.actions{display:flex;flex-wrap:wrap;gap:9px;margin-top:16px}button,.button-link{appearance:none;border:0;border-radius:12px;padding:12px 15px;background:linear-gradient(135deg,#9c88ff,#755cff);color:white;font:700 14px inherit;cursor:pointer;text-decoration:none;transition:transform .18s,filter .18s,opacity .18s}button:hover,.button-link:hover{transform:translateY(-1px);filter:brightness(1.08)}button:disabled{cursor:wait;opacity:.58;transform:none}.quiet{border:1px solid #3a466b;background:#202842;color:#d9e1fa}.tiny{margin:14px 0 0;color:#95a1bf;font-size:12px;line-height:1.55}.save-note{color:#85e8d5}.result-card{min-height:540px;display:flex;flex-direction:column}.letter{flex:1;min-height:300px;white-space:pre-wrap;border:1px solid #303b5f;border-radius:16px;background:#090d1b9c;padding:18px;color:#ecf0fc;font-size:15px;line-height:1.72}.letter.empty{display:flex;align-items:center;color:#94a0bf}.letter.error{color:#ffb1b7;border-color:#773b53}.letter.loading{color:#c5baff}.result-actions{margin-top:auto}.queue-card{margin-top:16px}.queue{display:grid;gap:10px}.queue-empty{padding:18px;border:1px dashed #3c476b;border-radius:14px;color:#96a1bf;font-size:14px}.queue-item{border:1px solid #344062;border-radius:15px;padding:14px;background:#0a0e1aa6}.queue-item strong{font-size:14px}.queue-item p{margin:7px 0 0;color:#aab4d0;font-size:13px}.queue-item .actions{margin-top:10px}.queue-item button{padding:9px 11px;font-size:12px}.notice{margin-top:16px;border-left:2px solid #56e5cf;padding:10px 12px;color:#abb6d2;background:#0d213026;border-radius:0 10px 10px 0;font-size:12px;line-height:1.55}.footer{margin-top:22px;color:#7e89a6;font-size:12px;line-height:1.55}.footer a{color:#a99bff}.hidden{display:none}@media(max-width:800px){.shell{padding:20px 15px 46px}.hero{padding:47px 0 27px}.workbench{grid-template-columns:1fr}.result-card{min-height:410px}.hero h1{font-size:45px}.nav .state{display:none}}@media(max-width:420px){.card{padding:18px}.actions{display:grid}.actions>*{width:100%}.hero h1{font-size:38px}}
</style></head><body><main class="shell"><nav class="nav"><div class="brand"><span class="mark">↗</span>Reply Studio</div><span class="state" id="profileState">профиль не сохранён</span></nav><header class="hero"><p class="eyebrow">Сопроводительные письма без AI-лимитов</p><h1>Хороший отклик.<br><span>Без лишних шагов.</span></h1><p class="lead">Вставьте краткое «о себе» и ссылку на вакансию. Письмо появится автоматически, останется у вас на проверку и будет готово к отправке в hh.ru.</p><div class="steps"><span class="step"><b>01</b> О себе</span><span class="step"><b>02</b> Ссылка hh.ru</span><span class="step"><b>03</b> Копировать и откликнуться</span></div></header><section class="workbench"><article class="card"><div class="section-head"><div><span class="index">01 · ВАШ КОНТЕКСТ</span><h2>Расскажите о себе</h2><p class="subtle">Достаточно 3–6 предложений: роль, стек, проект и сильный результат.</p></div></div><label class="field" for="profile"><span>Профиль <small>сохраняется только в этом браузере</small></span><textarea id="profile" placeholder="Например: AI Automation Engineer с опытом Go-разработки. Создаю REST API, интеграции и интерфейсы на React/TypeScript…"></textarea></label><div class="actions"><button id="saveProfile" type="button">Сохранить профиль</button></div><p id="saveNote" class="tiny">После сохранения его не нужно вставлять повторно.</p><div class="notice">Письмо строится локально по вашему тексту. Сервис не придумывает опыт и не получает доступ к аккаунту hh.ru.</div><div class="section-head" style="margin-top:28px;margin-bottom:0"><div><span class="index">02 · ВАКАНСИЯ</span><h2>Вставьте ссылку</h2><p class="subtle">После вставки корректной ссылки письмо формируется само.</p></div></div><label class="field" for="vacancyUrl"><span>Ссылка на вакансию hh.ru</span><input id="vacancyUrl" type="url" inputmode="url" placeholder="https://hh.ru/vacancy/…" autocomplete="url"></label><label class="field" for="vacancyTitle"><span>Название вакансии <small>необязательно, улучшает точность</small></span><input id="vacancyTitle" type="text" placeholder="Например: AI Automation Engineer"></label><label class="field" for="vacancyText"><span>Детали вакансии <small>необязательно</small></span><textarea id="vacancyText" style="min-height:110px" placeholder="Вставьте требования, если хотите учесть их в письме."></textarea></label><div class="actions"><button id="generate" type="button">Собрать письмо</button><a id="openVacancy" class="button-link quiet hidden" target="_blank" rel="noreferrer">Открыть вакансию</a></div></article><article class="card result-card"><div class="section-head"><div><span class="index">03 · ПРЕДПРОСМОТР</span><h2>Ваше письмо</h2><p class="subtle">Проверьте формулировки перед отправкой.</p></div></div><div id="result" class="letter empty">Здесь появится готовое письмо.</div><div class="actions result-actions"><button id="copyLetter" class="quiet" type="button">Копировать</button><button id="queueLetter" type="button">В очередь</button></div><p class="tiny">«В очередь» сохранит черновик и откроет вакансию только после вашего подтверждения.</p></article></section><section class="card queue-card"><div class="section-head"><div><span class="index">ЧЕРНОВИКИ</span><h2>Очередь на подтверждение</h2><p class="subtle">Никакой автоподачи: вы открываете вакансию и отправляете отклик сами.</p></div></div><div id="queue" class="queue"></div></section><p class="footer">Поиск вакансий внутри приложения отключён: официальный API hh.ru на этой сети возвращает защитную блокировку. Это нельзя и не нужно обходить. Ищите вакансии в <a href="https://hh.ru/search/vacancy" target="_blank" rel="noreferrer">hh.ru</a>, а сюда вставляйте ссылку на подходящую вакансию.</p></main><script>
const profile=document.querySelector('#profile'),vacancyUrl=document.querySelector('#vacancyUrl'),vacancyTitle=document.querySelector('#vacancyTitle'),vacancyText=document.querySelector('#vacancyText'),result=document.querySelector('#result'),generate=document.querySelector('#generate'),copyLetter=document.querySelector('#copyLetter'),queue=document.querySelector('#queue'),profileState=document.querySelector('#profileState'),saveNote=document.querySelector('#saveNote'),openVacancy=document.querySelector('#openVacancy');let profileTimer,linkTimer;
function safeJSON(key,fallback){try{return JSON.parse(localStorage.getItem(key)||'')}catch{return fallback}}
function savedProfile(){return localStorage.getItem('candidateProfile')||''}
function validHHURL(value){try{const url=new URL(value);return(url.protocol==='https:'||url.protocol==='http:')&&(url.hostname==='hh.ru'||url.hostname.endsWith('.hh.ru'))&&url.pathname.includes('/vacancy/')?url.href:''}catch{return ''}}
function setResult(text,kind=''){result.textContent=text;result.className='letter '+kind;result.dataset.letter=kind?'':text}
function saveProfile(silent=false){const value=profile.value.trim();if(!value){if(!silent){setResult('Добавьте несколько предложений о себе — это основа письма.','error')}return false}localStorage.setItem('candidateProfile',value);profileState.textContent='профиль сохранён локально';saveNote.textContent='Готово. Профиль останется на этом устройстве.';saveNote.className='tiny save-note';return true}
function updateVacancyLink(){const url=validHHURL(vacancyUrl.value.trim());openVacancy.classList.toggle('hidden',!url);if(url)openVacancy.href=url;return url}
async function createLetter(auto=false){const candidate=profile.value.trim(),url=updateVacancyLink(),title=vacancyTitle.value.trim(),details=vacancyText.value.trim();if(!candidate){setResult('Сначала добавьте краткое «о себе». Профиль сохранится автоматически.','error');return}if(vacancyUrl.value.trim()&&!url){setResult('Нужна полная ссылка вида https://hh.ru/vacancy/…','error');return}if(!url&&!title&&!details){setResult('Вставьте ссылку на hh.ru, название или текст вакансии.','error');return}saveProfile(true);generate.disabled=true;generate.textContent=auto?'Готовлю…':'Собираю…';setResult('Собираю письмо из ваших данных…','loading');try{const r=await fetch('/api/letter',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({candidate_profile:candidate,vacancy_title:title,vacancy_text:details||'Вакансия на hh.ru'})});const data=await r.json();if(!r.ok)throw new Error(data.error||'Не удалось собрать письмо');setResult(data.letter)}catch(error){setResult(error.message||'Не удалось соединиться с приложением.','error')}finally{generate.disabled=false;generate.textContent='Собрать письмо'}}
function items(){return safeJSON('autoApplyQueue',[])}
function renderQueue(){const list=items();queue.replaceChildren();if(!list.length){const empty=document.createElement('div');empty.className='queue-empty';empty.textContent='Пока пусто. Добавьте подготовленное письмо в очередь.';queue.append(empty);return}list.forEach((item,index)=>{const card=document.createElement('article');card.className='queue-item';const name=document.createElement('strong');name.textContent=item.title||'Вакансия на hh.ru';const caption=document.createElement('p');caption.textContent='Письмо готово и ждёт вашего подтверждения.';const actions=document.createElement('div');actions.className='actions';const approve=document.createElement('button');approve.type='button';approve.textContent='Открыть и скопировать';approve.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(item.letter)}catch{}window.open(item.url,'_blank','noopener');approve.textContent='Открыто'});const remove=document.createElement('button');remove.type='button';remove.className='quiet';remove.textContent='Убрать';remove.addEventListener('click',()=>{const next=items();next.splice(index,1);localStorage.setItem('autoApplyQueue',JSON.stringify(next));renderQueue()});actions.append(approve,remove);card.append(name,caption,actions);queue.append(card)})}
profile.value=savedProfile();if(profile.value){profileState.textContent='профиль сохранён локально';saveNote.textContent='Профиль восстановлен с этого устройства.';saveNote.className='tiny save-note'}renderQueue();
document.querySelector('#saveProfile').addEventListener('click',()=>{if(saveProfile()){setResult('Профиль сохранён. Теперь вставьте ссылку на вакансию.','empty')}});
profile.addEventListener('input',()=>{clearTimeout(profileTimer);profileTimer=setTimeout(()=>{if(profile.value.trim())saveProfile(true)},450)});
vacancyUrl.addEventListener('input',()=>{clearTimeout(linkTimer);const url=updateVacancyLink();if(!vacancyUrl.value.trim())return;if(!url){setResult('Проверьте ссылку: она должна вести на страницу вакансии hh.ru.','error');return}setResult('Ссылка принята. Готовлю письмо…','loading');linkTimer=setTimeout(()=>createLetter(true),350)});
generate.addEventListener('click',()=>createLetter());
copyLetter.addEventListener('click',async()=>{const letter=result.dataset.letter||'';if(!letter){setResult('Сначала соберите письмо.','error');return}try{await navigator.clipboard.writeText(letter);copyLetter.textContent='Скопировано';setTimeout(()=>copyLetter.textContent='Копировать',1600)}catch{setResult('Не удалось скопировать автоматически. Выделите текст вручную.','error')}});
document.querySelector('#queueLetter').addEventListener('click',()=>{const letter=result.dataset.letter||'',url=validHHURL(vacancyUrl.value.trim());if(!letter){setResult('Сначала соберите письмо.','error');return}if(!url){setResult('Для очереди добавьте корректную ссылку на hh.ru.','error');return}const list=items();if(list.some(item=>item.url===url)){setResult('Эта вакансия уже в очереди.','notice');return}list.push({url,title:vacancyTitle.value.trim(),letter});localStorage.setItem('autoApplyQueue',JSON.stringify(list));renderQueue();setResult('Черновик добавлен в очередь. Откройте вакансию, когда будете готовы.','notice');});
</script></body></html>`))
