package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type config struct {
	HHUserAgent string
}

type letterRequest struct {
	CandidateProfile string `json:"candidate_profile"`
	VacancyText      string `json:"vacancy_text"`
}

type letterResponse struct {
	Letter string `json:"letter"`
	Source string `json:"source"`
}

func main() {
	cfg := config{HHUserAgent: os.Getenv("HH_USER_AGENT")}
	if cfg.HHUserAgent == "" {
		cfg.HHUserAgent = "HH-Auto-Apply/1.0 (local)"
	}

	vacancies := newVacancyClient(cfg.HHUserAgent, &http.Client{Timeout: 20 * time.Second})

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/letter", handleLetter)
	mux.HandleFunc("/api/vacancies", vacancies.handleVacancies)

	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Cover Letter Studio is running at http://localhost%s", addr)
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
	if input.CandidateProfile == "" || input.VacancyText == "" {
		writeError(w, http.StatusBadRequest, "заполните профиль кандидата и текст вакансии")
		return
	}

	writeJSON(w, http.StatusOK, letterResponse{Letter: buildTemplateLetter(input), Source: "local_template"})
}

func buildTemplateLetter(input letterRequest) string {
	title := firstLine(input.VacancyText, 120)
	summary := firstSentence(input.CandidateProfile, 360)
	return "Здравствуйте!\n\nЗаинтересовала вакансия «" + title + "». В моём профиле указан следующий релевантный опыт: " + summary + "\n\nБуду рад(а) подробнее обсудить, чем могу быть полезен(на) команде. Спасибо за рассмотрение моей кандидатуры."
}

func firstLine(text string, limit int) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "Вакансия:"))
		if line != "" {
			return truncateText(line, limit)
		}
	}
	return "в вашей компании"
}

func firstSentence(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if index := strings.IndexAny(text, ".!?"); index >= 0 {
		text = text[:index+1]
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

func compactError(raw []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.Error.Message != "" {
		return payload.Error.Message
	}
	return strings.TrimSpace(string(raw))
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
<title>Cover Letter Studio</title><style>
:root{font-family:Inter,ui-sans-serif,system-ui,sans-serif;color:#e8edf8;background:#0d1220}body{margin:0}.wrap{max-width:1100px;margin:auto;padding:40px 20px 64px}h1{margin:0;font-size:clamp(30px,5vw,48px)}.lead{color:#afbdd7;max-width:780px;line-height:1.55}.grid{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin-top:28px}.card{background:#151d30;border:1px solid #293550;border-radius:16px;padding:20px}label{display:block;font-weight:650;margin-bottom:8px}textarea,input,select{width:100%;box-sizing:border-box;border-radius:10px;border:1px solid #3a4967;padding:13px;background:#0d1423;color:#e8edf8;font:14px/1.5 inherit}textarea{height:350px;resize:vertical}button{margin-top:20px;border:0;border-radius:10px;padding:13px 18px;background:#7c5cff;color:#fff;font:600 15px inherit;cursor:pointer}button.secondary{background:#273552;margin-left:8px}button:disabled{opacity:.6;cursor:wait}.note{color:#aab8d1;font-size:13px;line-height:1.5}.result{white-space:pre-wrap;min-height:185px;line-height:1.6}.error{color:#ffb4b4}.top{display:flex;justify-content:space-between;gap:16px;align-items:flex-start}.automation{margin-top:24px;display:flex;align-items:center;justify-content:space-between;gap:16px}.badge{display:inline-block;color:#d8e2ff;background:#202d4a;border:1px solid #3a4c73;border-radius:99px;padding:5px 10px;font-size:13px}.modal{position:fixed;inset:0;background:#060a13cc;display:flex;align-items:center;justify-content:center;padding:20px;z-index:2}.modal[hidden]{display:none}.dialog{width:min(680px,100%);max-height:calc(100vh - 40px);overflow:auto;background:#151d30;border:1px solid #3a4967;border-radius:18px;padding:24px}.choices{display:grid;gap:10px;margin:16px 0}.choice{display:block;border:1px solid #3a4967;border-radius:12px;padding:14px;cursor:pointer}.choice:has(input:checked){border-color:#8d78ff;background:#1e1b42}.choice input{width:auto;margin-right:8px}.rules{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:16px}.rules .wide{grid-column:1/-1}.privacy{border-left:3px solid #7c5cff;padding-left:12px}.hidden{display:none}.hh-actions{display:flex;gap:8px;flex-wrap:wrap}.vacancy-list,.queue-list{display:grid;gap:10px;margin-top:14px}.vacancy{border:1px solid #33425f;border-radius:12px;padding:14px}.vacancy h3{margin:0 0 5px;font-size:16px}.vacancy p{margin:7px 0}.vacancy button{margin-top:6px;padding:8px 12px;font-size:13px}.vacancy a{color:#b7aaff}.queue-empty{color:#aab8d1}@media(max-width:760px){.grid,.rules{grid-template-columns:1fr}.rules .wide{grid-column:auto}textarea{height:260px}.top,.automation{display:block}button.secondary{margin-left:0}.automation button{margin-top:12px}}
</style></head><body><main class="wrap"><div class="top"><div><h1>Cover Letter Studio</h1><p class="lead">Персональные письма и аккуратная очередь откликов на вакансии. Бот не отправляет ничего без выбранного вами режима и заданных ограничений.</p></div><span id="modeBadge" class="badge">Настройка не завершена</span></div><section class="card automation"><div><strong>Ассистент откликов</strong><p id="automationSummary" class="note">Укажите способ работы и фильтры, чтобы начать.</p><p id="hhStatus" class="note">Публичный поиск hh.ru готов. При требовании капчи откройте hh.ru и подтвердите поиск вручную.</p></div><div class="hh-actions"><button id="openSetup" type="button">Включить и настроить</button><button id="searchHH" class="secondary" type="button">Найти вакансии</button></div></section><section class="card" id="vacanciesCard" style="margin-top:20px" hidden><label>Подходящие вакансии</label><div id="vacancies" class="vacancy-list"></div></section><div class="grid"><section class="card"><label for="profile">Мой профиль</label><textarea id="profile" placeholder="Один раз вставьте проверенную выжимку: опыт, проекты, стек, образование, результаты."></textarea><button id="saveProfile" type="button">Сохранить профиль</button><p class="note">Профиль хранится только в браузере на этом компьютере. Первая фраза используется в письме дословно.</p></section><section class="card"><label for="vacancyUrl">Ссылка на вакансию hh.ru</label><input id="vacancyUrl" type="url" inputmode="url" placeholder="Вставьте https://hh.ru/vacancy/... — письмо соберётся автоматически" autocomplete="url"><label for="vacancy" style="margin-top:16px">Дополнительный текст вакансии</label><textarea id="vacancy" placeholder="Необязательно. Нужен только если хотите добавить детали вручную."></textarea><button id="generate">Собрать письмо вручную</button><button id="copyLetter" class="secondary" type="button">Копировать</button><button id="queueLetter" class="secondary" type="button">В очередь</button><p class="note">После сохранения профиля достаточно вставить ссылку: письмо собирается локально, без AI и лимитов.</p></section></div><section class="card" style="margin-top:20px"><label>Готовое сопроводительное письмо</label><div id="result" class="result note">Сначала сохраните профиль, затем вставьте ссылку на вакансию.</div></section><section class="card" style="margin-top:20px"><label>Очередь на подтверждение</label><p class="note">Нажмите «Одобрить и открыть hh.ru»: письмо скопируется, а вакансия откроется в новой вкладке. Финальный отклик отправляете вы.</p><div id="queue" class="queue-list"></div></section></main><div class="modal" id="setupModal" hidden><form class="dialog" id="setupForm"><h2>Как будет работать ассистент?</h2><p class="note">Рекомендую начать с очереди на подтверждение: вы видите вакансию, письмо и кнопку отправки до каждого отклика.</p><div class="choices"><label class="choice"><input type="radio" name="mode" value="drafts"> <strong>Только черновики</strong><br><span class="note">Собирает письмо по локальному шаблону, отклик вы отправляете сами.</span></label><label class="choice"><input type="radio" name="mode" value="review" checked> <strong>Очередь на подтверждение — рекомендовано</strong><br><span class="note">Подбирает вакансии и готовит письма, но каждый отклик подтверждаете вы.</span></label><label class="choice"><input type="radio" name="mode" value="automatic"> <strong>Автоподача по строгим правилам</strong><br><span class="note">Недоступна для аккаунтов соискателей: перед откликом всегда требуется ваше действие в hh.ru.</span></label></div><div class="rules"><label class="wide">Ищу позиции<input id="queries" required placeholder="Например: Go developer, automation engineer"></label><label>Города / формат<input id="locations" placeholder="Минск, удалённо"></label><label>ID региона hh.ru<input id="hhArea" inputmode="numeric" placeholder="Например: 1002 для Минска"></label><label>Минимальная зарплата<input id="salary" inputmode="numeric" placeholder="Например: 2500"></label><label>Макс. откликов в день<input id="dailyLimit" type="number" min="1" max="30" value="8" required></label><label class="wide">Обязательные навыки<input id="includeKeywords" placeholder="Go, PostgreSQL, n8n"></label><label class="wide">Исключить вакансии<input id="excludeKeywords" placeholder="стажировка, холодные продажи"></label></div><p class="note privacy">Настройки и очередь остаются только в браузере на этом компьютере. Бот не получает пароль hh.ru и не отправляет отклики.</p><button type="submit">Сохранить режим</button><button type="button" class="secondary" id="closeSetup">Отмена</button></form></div><script>
const button=document.querySelector('#generate'),result=document.querySelector('#result'),modal=document.querySelector('#setupModal'),setupForm=document.querySelector('#setupForm'),modeBadge=document.querySelector('#modeBadge'),automationSummary=document.querySelector('#automationSummary'),hhStatus=document.querySelector('#hhStatus'),queue=document.querySelector('#queue'),profile=document.querySelector('#profile'),vacancyUrl=document.querySelector('#vacancyUrl');
const modes={drafts:'Только черновики',review:'Очередь на подтверждение',automatic:'Автоподача недоступна для соискателя'};let selectedVacancy=null;
function settings(){try{return JSON.parse(localStorage.getItem('autoApplySettings')||'null')}catch{return null}}
function savedProfile(){return localStorage.getItem('candidateProfile')||''}
function renderSettings(){const s=settings();if(!s)return;modeBadge.textContent=modes[s.mode];automationSummary.textContent=s.queries+' · до '+s.dailyLimit+' откликов в день'+(s.locations?' · '+s.locations:'');document.querySelector('#openSetup').textContent='Изменить настройки'}
function splitWords(value){return(value||'').toLowerCase().split(',').map(x=>x.trim()).filter(Boolean)}
function matchesRules(v,s){const text=(v.name+' '+v.employer.name+' '+v.snippet.requirement+' '+v.snippet.responsibility).toLowerCase(),required=splitWords(s.includeKeywords),excluded=splitWords(s.excludeKeywords),min=Number(s.salary||0),salary=v.salary&&Math.max(v.salary.from||0,v.salary.to||0);return required.every(word=>text.includes(word))&&!excluded.some(word=>text.includes(word))&&(!min||salary>=min)}
function openSetup(){const s=settings();if(s){setupForm.mode.value=s.mode;['queries','locations','hhArea','salary','dailyLimit','includeKeywords','excludeKeywords'].forEach(id=>document.querySelector('#'+id).value=s[id]||'')}modal.hidden=false}
function hhURL(value){try{const url=new URL(value);return(url.protocol==='https:'||url.protocol==='http:')&&(url.hostname==='hh.ru'||url.hostname.endsWith('.hh.ru'))?url.href:''}catch{return ''}}
function renderQueue(){const items=JSON.parse(localStorage.getItem('autoApplyQueue')||'[]');queue.replaceChildren();if(!items.length){queue.textContent='Очередь пока пуста.';queue.className='queue-list queue-empty';return}queue.className='queue-list';items.forEach((item,index)=>{const card=document.createElement('article');card.className='vacancy';const title=document.createElement('strong');title.textContent=item.name;const company=document.createElement('p');company.className='note';company.textContent=item.company+' · ждёт вашего подтверждения';const letter=document.createElement('p');letter.className='note';letter.textContent=item.letter;const remove=document.createElement('button');remove.type='button';remove.className='secondary';remove.textContent='Убрать';remove.addEventListener('click',()=>{items.splice(index,1);localStorage.setItem('autoApplyQueue',JSON.stringify(items));renderQueue()});card.append(title,company,letter);if(item.url){const approve=document.createElement('button');approve.type='button';approve.textContent='Одобрить и открыть hh.ru';approve.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(item.letter)}catch{}window.open(item.url,'_blank','noopener');approve.textContent='Открыто — вставьте письмо'});card.append(approve)}else{const missing=document.createElement('p');missing.className='note';missing.textContent='Добавьте ссылку hh.ru, чтобы открыть вакансию из очереди.';card.append(missing)}card.append(remove);queue.append(card)})}
function prepareVacancy(v){selectedVacancy=v;document.querySelector('#vacancyUrl').value=v.alternate_url||'';const details=[v.name,v.employer.name,v.area.name,v.snippet.requirement,v.snippet.responsibility].filter(Boolean).join('\n\n');document.querySelector('#vacancy').value=details;document.querySelector('#vacancy').scrollIntoView({behavior:'smooth',block:'center'})}
function renderVacancies(items){const box=document.querySelector('#vacancies'),card=document.querySelector('#vacanciesCard');box.replaceChildren();card.hidden=false;if(!items.length){box.textContent='По заданным фильтрам вакансий не найдено.';return}items.forEach(v=>{const row=document.createElement('article');row.className='vacancy';const title=document.createElement('h3');title.textContent=v.name;const meta=document.createElement('p');meta.className='note';meta.textContent=[v.employer.name,v.area.name].filter(Boolean).join(' · ');const snippet=document.createElement('p');snippet.className='note';snippet.textContent=v.snippet.requirement||v.snippet.responsibility||'Описание в карточке вакансии';const pick=document.createElement('button');pick.type='button';pick.textContent='Подготовить письмо';pick.addEventListener('click',()=>prepareVacancy(v));const link=document.createElement('a');link.href=v.alternate_url;link.target='_blank';link.rel='noreferrer';link.textContent='Открыть в hh.ru';row.append(title,meta,snippet,pick,document.createTextNode(' '),link);box.append(row)})}
document.querySelector('#openSetup').addEventListener('click',openSetup);document.querySelector('#closeSetup').addEventListener('click',()=>modal.hidden=true);modal.addEventListener('click',e=>{if(e.target===modal)modal.hidden=true});
setupForm.addEventListener('submit',e=>{e.preventDefault();const values=new FormData(setupForm),s={mode:values.get('mode')};['queries','locations','hhArea','salary','dailyLimit','includeKeywords','excludeKeywords'].forEach(id=>s[id]=document.querySelector('#'+id).value.trim());localStorage.setItem('autoApplySettings',JSON.stringify(s));modal.hidden=true;renderSettings()});renderSettings();renderQueue();
document.querySelector('#searchHH').addEventListener('click',async()=>{const s=settings();if(!s||!s.queries){openSetup();return}const search=document.querySelector('#searchHH');search.disabled=true;search.textContent='Ищу…';try{const r=await fetch('/api/vacancies',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({text:s.queries.split(',')[0].trim(),area:s.hhArea||'',per_page:20})}),data=await r.json();if(!r.ok)throw new Error(data.error||'Ошибка поиска');renderVacancies(data.items.filter(v=>matchesRules(v,s)));hhStatus.textContent='Найдено '+data.found+' вакансий; показаны только прошедшие ваши фильтры.'}catch(e){hhStatus.textContent=e.message}finally{search.disabled=false;search.textContent='Найти вакансии'}});
if(savedProfile())profile.value=savedProfile();
document.querySelector('#saveProfile').addEventListener('click',()=>{const value=profile.value.trim();if(!value){result.textContent='Вставьте проверенную выжимку профиля, затем сохраните её.';result.className='result error';return}localStorage.setItem('candidateProfile',value);result.textContent='Профиль сохранён локально. Теперь достаточно вставить ссылку на вакансию.';result.className='result'});
async function createLetter(candidate_profile,vacancy_text){if(!candidate_profile.trim()||!vacancy_text.trim()){result.textContent='Сначала сохраните профиль и вставьте ссылку или текст вакансии.';result.className='result error';return}button.disabled=true;button.textContent='Собираю…';result.textContent='';result.className='result note';try{const r=await fetch('/api/letter',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({candidate_profile,vacancy_text})});const data=await r.json();if(!r.ok)throw new Error(data.error||'Ошибка запроса');result.textContent=data.letter;result.className='result'}catch(e){result.textContent=e.message;result.className='result error'}finally{button.disabled=false;button.textContent='Собрать письмо вручную'}}
button.addEventListener('click',()=>createLetter(profile.value,document.querySelector('#vacancy').value));
let linkTimer;vacancyUrl.addEventListener('input',()=>{clearTimeout(linkTimer);linkTimer=setTimeout(()=>{const url=hhURL(vacancyUrl.value.trim()),candidate_profile=savedProfile();if(!url)return;if(!candidate_profile){result.textContent='Сначала один раз сохраните профиль слева.';result.className='result error';return}selectedVacancy=null;const vacancy_text=document.querySelector('#vacancy').value.trim()||'Вакансия на hh.ru';createLetter(candidate_profile,vacancy_text)},450)});
document.querySelector('#copyLetter').addEventListener('click',async()=>{if(!result.textContent||result.classList.contains('error'))return;try{await navigator.clipboard.writeText(result.textContent);document.querySelector('#copyLetter').textContent='Скопировано';setTimeout(()=>document.querySelector('#copyLetter').textContent='Копировать',1500)}catch{result.textContent='Не удалось скопировать текст. Выделите его вручную.';result.className='result error'}});
document.querySelector('#queueLetter').addEventListener('click',()=>{if(!result.textContent||result.classList.contains('error')){result.textContent='Сначала заполните профиль, вакансию и соберите письмо.';result.className='result error';return}const urlText=document.querySelector('#vacancyUrl').value.trim(),url=urlText?hhURL(urlText):'';if(urlText&&!url){result.textContent='Укажите корректную ссылку на hh.ru.';result.className='result error';return}const id=selectedVacancy?selectedVacancy.id:(url||'manual-'+Date.now()),name=selectedVacancy?selectedVacancy.name:document.querySelector('#vacancy').value.split('\n')[0].trim(),company=selectedVacancy?selectedVacancy.employer.name:'Вакансия по ссылке';const items=JSON.parse(localStorage.getItem('autoApplyQueue')||'[]');if(items.some(item=>item.id===id)){result.textContent='Эта вакансия уже находится в очереди.';return}items.push({id,name:name||'Вакансия',company,letter:result.textContent,url});localStorage.setItem('autoApplyQueue',JSON.stringify(items));renderQueue();result.textContent=url?'Добавлено в очередь. Одобрите вакансию ниже, когда будете готовы.':'Добавлено в очередь. Добавьте ссылку hh.ru, чтобы открывать вакансию из очереди.';result.className='result'});
</script></body></html>`))
