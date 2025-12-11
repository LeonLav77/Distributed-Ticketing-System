# **Distribuirana prodaja koncertnih karata**

## **Tehnička dokumentacija**

## **1\. Infrastruktura**

Infrastruktura projekta napravljena je u mikroservisnom stilu. Čine ju 8 mikroservisa napisanih u GO programskom jeziku, 2 eksterna servisa, 2 zasebne Redis instance, ETCD roj, PostgreSQL baze podataka, te RabbitMQ posrednik poruka.

**Mikroservisi:**

1. CDN Service \- Posluživanje statičkih datoteka  
2. Auth Service \- Autentifikacija korisnika  
3. Data Aggregator Service \- Agregacija podataka iz više izvora  
4. Ticket Purchase Service \- Rezervacija i kupnja karata  
5. Websocket Loadbalancer \- Distribucija WS konekcija  
6. Websocket Server Service \- Održavanje reda čekanja  
7. Queue Control Service \- Admin upravljanje redom  
8. Order Service \- Obrada narudžbi

**Eksterni servisi:**

1. Directus CMS \- Upravljanje sadržajem  
2. Payment Processor \- Obrada plaćanja

<img width="1921" height="1161" alt="Untitled Diagram-Microservices Architecture drawio (1)" src="https://github.com/user-attachments/assets/7b96907d-9ece-4475-9a37-cb2d5c3c2afe" />

## **2\. Distribuirani pristup**

### **2.1 Skaliranje sistema**

Sistem koristi distribuirani pristup kako bi riješio problem visoke konkurentnosti pri prodaji karata. Najveće opterećenje očekuje se u redu čekanja za određeni koncert, zato je postoje:

* 3 Websocket poslužitelja za red čekanja  
* Load balancer za distribuciju opterećenja

Ovaj pristup omogućava horizontalno skaliranje \- dodavanje novih instanci servisa po potrebi bez zaustavljanja postojećih. Pad jedne instance ne zaustavlja cijeli sistem.

### **2.2 ETCD za upravljanje kartama**

ETCD je odabran kao distribuirana key-value baza za spremanje trenutnog stanja dostupnih karata. Koristi Raft konsenzus algoritam gdje svi nodovi moraju postići dogovor o stanju podataka. Koristi se u distribuiranom sustavu zbog otpornosti na pad jednog servisa, te rad unatoč tome

**Rezervacija karata s optimističnim transakcijama:**

go

*// Dohvati trenutno stanje karata*

availableTickets := etcdClient.Get(key)

*// Provjeri ima li dovoljno karata*

if currentInt \< requestAmount { return error }

*// Rezerviraj samo ako se stanje nije promijenilo*

etcdClient.Txn(ctx).

    If(Compare(Version(key), "=", version)).

    Then(Put(key, newValue)).

    Commit()

Compare-And-Swap (CAS) operacija osigurava da više korisnika ne može istovremeno rezervirati iste karte. Ako se broj karata promijeni između čitanja i pisanja, transakcija se odbija i pokušava ponovno. Pokušava se n puta koji je u slučaju ovog sustava postavljen na 10 u varijablama okruženja

## **3\. Redis za caching i queue**

### **3.1 Tickets Redis**

Redis se koristi kao cache layer ispred ETCD-a za ubrzanje read operacija. Konfiguriran je sa TTL-om od 10 sekundi:

go

key := fmt.Sprintf("event:%s:available\_tickets:%s", eventId, ticketType)

availableTickets, err := rdb.Get(ctx, key).Result()

if err \== redis.Nil {

    *// Cache miss \- dohvati iz ETCD*

    quantity := getAvailableTicketsFromEtcd(eventId, ticketType)

    *// Spremi u Redis sa TTL-om*

    rdb.Set(ctx, key, quantity, 10\*time.Second)

}

Prvi zahtjev dohvaća podatke iz ETCD-a i sprema ih u Redis. Sljedeći zahtjevi u roku od 10 sekundi vraćaju podatke iz Redisa bez opterećivanja ETCD clustera.

### **3.2 Websocket Redis**

Redis Sorted Set struktura koristi se za održavanje reda čekanja korisnika:

go

*// Dodaj korisnika u red sa timestampom kao score*

ZADD ws\-queue:eventID timestamp userID

*// Dohvati poziciju korisnika u redu*

ZRANK ws\-queue:eventID userID

*// Dohvati ukupan broj korisnika u redu*

ZCARD ws\-queue:eventID

Sorted Set automatski održava sortirani red po timestampu dodavanja. Sve operacije su atomske i thread-safe, što omogućava da više WebSocket servera istovremeno upravlja redom bez race conditiona.

## 4\. RabbitMQ message broker

### 4.1 Asinkrona komunikacija između servisa

RabbitMQ omogućava odvajanje servisa kroz asinkronu razmjenu poruka. Servisi ne moraju biti dostupni istovremeno \- poruke se čuvaju u redu čekanja dok potrošač ne bude spreman za obradu.

Deklarirani queue-ovi:

\- \`order.created\` \- kreiranje nove narudžbe

\- \`order.payment\-success\` \- uspješna plaćanje

### 4.2 Životni ciklus narudžbe

Kreiranje narudžbe

1. Ticket Service rezervira karte  
2. Šalje poruku u order.created red čekanja  
3. Order Service prima poruku  
4. Kreira nardužbu u PostgreSQL bazi  
5. Šalje ACK poruku u RabbitMQ

Završetak narudžbe:

1. Payment processor šalje webhook  
2. Ticket Service prima webhook  
3. Šalje poruku u order.payment-success red čekanja  
4. Order Service ažurira status narudžber  
5. Šalje ACK poruku u RabbitMQ

RabbitMQ garantira at-least-once delivery \- poruka će biti isporučena barem jednom, ali može biti isporučena više puta ako consumer padne prije ACK-a.

<img width="621" height="371" alt="Untitled Diagram-RabbitMQ drawio" src="https://github.com/user-attachments/assets/db914038-ff30-48e4-9fe6-034c976f99e0" />

### **4.3 Idempotentna obrada poruka**

Order Service implementira provjeru duplikata kako bi osigurao da se poruke mogu sigurno procesuirati više puta bez dupliciranja podataka:

go

*// Provjeri postoji li već narudžba*

checkQuery := \`SELECT id FROM orders 

               WHERE order\_reference\_id \= $1\`

err := db.QueryRow(checkQuery, data.OrderReferenceId).Scan(&existingOrderID)

if err \== nil {

    *// Narudžba već postoji*

    message.Ack(false)

    continue

}

*// Kreiraj novu narudžbu*

Ako Order Service primi istu poruku dvaput, drugi put će samo potvrditi poruku bez kreiranja duplikata u bazi.

## **5\. Exactly-once semantika**

### **5.1 CUID identifikatori**

Svaka nardužba dobiva CUID (Collision-resistant Unique Identifier) kao jedinstveni referentni broj:

go

orderReferenceId := cuid.New()

*// Generira: "cjld2cyuq0000t3rmniod1foy"*

CUID kombinira zapis vremena, brojač, otisak poslužitelja i nasumične vrijednosti. Ovo garantira jedinstvenost bez potrebe za koordinacijom između servisa ili konzultacije baze podataka.

### **5.2 Stege baze podataka**

PostgreSQL UNIQUE stega sprječava kreiranje duplikata na razini baze:

sql

CREATE TABLE orders (

    id SERIAL PRIMARY KEY,

    order\_reference\_id VARCHAR(255) UNIQUE NOT NULL,

    user\_id INTEGER NOT NULL,

    event\_id VARCHAR(255) NOT NULL,

    total\_quantity INTEGER NOT NULL,

    status VARCHAR(50) DEFAULT 'pending'

);

Kombinacija CUIDa i stege na bazi podataka osigurava exactly-once semantiku:

1. CUID osigurava da svaki request ima jedinstven ID  
2. Database stega odbija INSERT ako ID već postoji  
3. Aplikacija hvata “duplicate error” i ACK-a poruku bez kreiranja narudžbe

go

err \= tx.QueryRow(query, ...).Scan(&orderID)

if err \!= nil {

    if pqErr, ok := err.(\*pq.Error); ok {

        if string(pqErr.Code) \== "23505" { *// Duplicate*

            tx.Rollback()

            message.Ack(false)

            continue

        }

    }

}

\`\`\`

## **6\. Mreža**

### **6.1 Arhitektura mreže**

Sistem je organiziran u slojeve prema tipu komunikacije i pristupa:

**Eksterni sloj:**

* CDN Service izložen na portu 8080  
* Jedina točka ulaza u sistem za korisnike

**Aplikacijski sloj:**

* Auth Service \- autentifikacija  
* Ticket Purchase Service \- rezervacije  
* Data Aggregator Service \- agregacija podataka  
* WebSocket Load Balancer \- distribucija WS konekcija  
* WebSocket Server instances (3x) \- održavanje redova

**Sloj podataka:**

* PostgreSQL \- trajno spremanje podataka  
* ETCD cluster (3 čvora) \- distribuirana baza za karte  
* Redis instances (2x) \- cache i queue management  
* RabbitMQ \- message broker

Samo CDN Service ima direktnu komunikaciju s preglendikom. Svi ostali servisi komuniciraju interno kroz privatnu mrežu.

## **7\. Autentifikacija**

### **7.1 JWT tokeni**

Sistem koristi dva tipa JWT tokena:

**Auth token** \- generira Auth Service nakon prijave kao dokaz o prijavi korisnika:

claims := &UserClaims{  
 	Username: username,   
	UserID: userID,   
	RegisteredClaims: jwt.RegisteredClaims{   
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 \* time.Hour)),   
	},   
}   
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

**Admission token** \- generira WebSocket Service kada korisnik dođe na red:

claims := &AdmissionsClaims{   
	EventID: eventID,   
	UserID: userID,   
	RegisteredClaims: jwt.RegisteredClaims{   
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 \* time.Hour)),   
},   
}   
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

Admission token služi kao dokaz da je korisnik prošao kroz red čekanja i ima pravo pristupa stranici za kupnju karata.

### **7.2 Password hashing**

Auth Service koristi bcrypt sa dodatnim salt-om za čuvanje lozinki:

salt := generateSalt(16) hashedPassword  
err := bcrypt.GenerateFromPassword(  
	\[\]byte(password \+ salt),   
	bcrypt.DefaultCost  
)

Salt se generira za svakog korisnika i sprema se u bazu zajedno sa hashom. Pri loginu se lozinka kombinira sa saltom prije usporedbe:

err \= bcrypt.CompareHashAndPassword(   
	\[\]byte(user.Password \+ user.Salt),   
	\[\]byte(credentials.Password\+user.Salt)   
)

## 8\. Kompletni flow sustava:

<img width="871" height="811" alt="Untitled Diagram-Working Principle drawio (1)" src="https://github.com/user-attachments/assets/69e72541-81fc-437e-a9e1-ec51ce2d6672" />



## 9\. Zasebno postavljanje projekta

Postavljanje projekta u lokalnom okruženju je jednostavno zahvaljujući Docker i Docker Compose alatima. Potrebno ih je instalirati kako bi se mogao pratiti sljedeći proces postavljanja:

1. Potvrditi lokalni rad već navedenih alata Docker i Docker Compose  
2. Pronaći i preuzeti “docker-compose.yml” datoteku sa github repozitorija: [https://github.com/LeonLav77/Distributed-Ticketing-System](https://github.com/LeonLav77/Distributed-Ticketing-System)  
   (curl \-O https://raw.githubusercontent.com/leonlav77/Distributed-Ticketing-System/main/docker-compose.yml)  
3. Na putanji na kojoj se nalazi ta datoteka izvršiti sljedeću komandu:   
   “docker compose up”  
4. Pričekati nekoliko trenutaka  
5. Potvrditi učitavanje mrežne stranice na adresi: “127.0.0.1:8080” (ili nekoj drugoj ukoliko je promijenjeno okruženje)

U trenutku kada je mrežna stranica aktivna, postavljanje je uspješno. Za dodatno testiranje sustava predloženo je:

1. Provjeriti adresu mrežnu stranicu na adresi “127.0.0.1:9000” → korištena za testiranje reda čekanja za određene koncerte  
2. Probati ući u red čekanja za određeni koncert  
3. Kreirati profil i prijaviti se s tim profilom  
4. Dohvatiti ID nekog određenog koncerta (npr. 58b85029-af94-498e-ae3a-2fda2b5d6c5a) te izvršiti komandu koja se također nalazi u [README.md](http://README.md) datoteci u repozitoriju:  
   “docker compose exec etcd\_service\_1 etcdctl put "concert:58b85029-af94-498e-ae3a-2fda2b5d6c5a:available:basic" "10000"”  
5. Odraditi kompletnu narudžbu, te provjeriti profil
