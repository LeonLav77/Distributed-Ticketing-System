# **Distribuirana prodaja koncertnih karata**
## **Tehnička dokumentacija**

---

## **1. Infrastruktura**

Infrastruktura projekta napravljena je u mikroservisnom stilu. Čine ju 8 mikroservisa napisanih u GO programskom jeziku, 2 eksterna servisa, 2 zasebne Redis instance, ETCD roj, PostgreSQL baze podataka te RabbitMQ posrednik poruka.

### **Mikroservisi**

1. CDN Service – Posluživanje statičkih datoteka  
2. Auth Service – Autentifikacija korisnika  
3. Data Aggregator Service – Agregacija podataka iz više izvora  
4. Ticket Purchase Service – Rezervacija i kupnja karata  
5. Websocket Loadbalancer – Distribucija WS konekcija  
6. Websocket Server Service – Održavanje reda čekanja  
7. Queue Control Service – Admin upravljanje redom  
8. Order Service – Obrada narudžbi

### **Eksterni servisi**

1. Directus CMS – Upravljanje sadržajem  
2. Payment Processor – Obrada plaćanja

---

## **2. Distribuirani pristup**

### **2.1 Skaliranje sistema**

Sistem koristi distribuirani pristup kako bi riješio problem visoke konkurentnosti pri prodaji karata.

Postoje:

- 3 WebSocket poslužitelja  
- 1 Load balancer  

Omogućeno je horizontalno skaliranje i tolerancija pada pojedine instance.

### **2.2 ETCD za upravljanje kartama**

ETCD koristi Raft konsenzus algoritam kako bi svi čvorovi imali konzistentno stanje.

#### **Rezervacija karata s optimističnim transakcijama**

```go
// Dohvati trenutno stanje karata
availableTickets := etcdClient.Get(ctx, key)

// Provjeri ima li dovoljno karata
if currentInt < requestAmount {
    return error
}

// Rezerviraj samo ako se stanje nije promijenilo
etcdClient.Txn(ctx).
    If(Compare(Version(key), "=", version)).
    Then(Put(key, newValue)).
    Commit()
```

Ako je stanje promijenjeno, transakcija se odbija i pokušava ponovno (max 10 pokušaja).

---

## **3. Redis za caching i queue**

### **3.1 Tickets Redis**

Redis služi kao cache ispred ETCD-a:

```go
key := fmt.Sprintf("event:%s:available_tickets:%s", eventId, ticketType)

availableTickets, err := rdb.Get(ctx, key).Result()

if err == redis.Nil {
    // Cache miss – dohvati iz ETCD
    quantity := getAvailableTicketsFromEtcd(eventId, ticketType)

    // Spremi u Redis sa TTL-om
    rdb.Set(ctx, key, quantity, 10*time.Second)
}
```

---

### **3.2 Websocket Redis**

Redis Sorted Set za red čekanja:

```go
// Dodaj korisnika u red sa timestampom kao score
ZADD ws-queue:eventID timestamp userID

// Dohvati poziciju korisnika
ZRANK ws-queue:eventID userID

// Dohvati broj korisnika u redu
ZCARD ws-queue:eventID
```

---

## **4. RabbitMQ message broker**

### **4.1 Asinkrona komunikacija**

Deklarirani queueovi:

- `order.created`
- `order.payment-success`

---

### **4.2 Životni ciklus narudžbe**

1. Ticket Service rezervira karte  
2. Šalje poruku u `order.created`  
3. Order Service kreira narudžbu  
4. Šalje ACK  

Završetak narudžbe:

1. Payment processor šalje webhook  
2. Ticket Service prima webhook  
3. Šalje poruku `order.payment-success`  
4. Order Service ažurira narudžbu  
5. Šalje ACK  

---

### **4.3 Idempotentna obrada poruka**

```go
// Provjeri postoji li narudžba
checkQuery := `
SELECT id FROM orders
WHERE order_reference_id = $1
`

err := db.QueryRow(checkQuery, data.OrderReferenceId).Scan(&existingOrderID)

if err == nil {
    // Narudžba postoji
    message.Ack(false)
    continue
}
```

---

## **5. Exactly-once semantika**

### **5.1 CUID identifikatori**

```go
orderReferenceId := cuid.New()
```

### **5.2 UNIQUE constraint**

```sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    order_reference_id VARCHAR(255) UNIQUE NOT NULL,
    user_id INTEGER NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    total_quantity INTEGER NOT NULL,
    status VARCHAR(50) DEFAULT 'pending'
);
```

---

## **6. Mreža**

### **6.1 Arhitektura mreže**

Sustav je podijeljen u tri sloja:

- **Eksterni sloj** – CDN  
- **Aplikacijski sloj** – API servisi i WebSocket  
- **Sloj podataka** – Redis, ETCD, RabbitMQ, PostgreSQL  

---

## **7. Autentifikacija**

### **7.1 JWT tokeni**

#### **Auth token**

```go
claims := &UserClaims{
    Username: username,
    UserID: userID,
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
    },
}
```

#### **Admission token**

```go
claims := &AdmissionsClaims{
    EventID: eventID,
    UserID: userID,
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
    },
}
```

---

### **7.2 Password hashing**

```go
salt := generateSalt(16)
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.DefaultCost)
```

Validacija:

```go
bcrypt.CompareHashAndPassword(
    []byte(user.Password),
    []byte(credentials.Password + user.Salt),
)
```

---

## **8. Kompletni flow sustava**

(izostavljene slike ostaju u originalnom dokumentu)

---

## **9. Zasebno postavljanje projekta**

### **9.1 Lokalno pokretanje**

```sh
curl -O https://raw.githubusercontent.com/leonlav77/Distributed-Ticketing-System/main/docker-compose.yml
docker compose up
```

---

### **9.2 Testiranje**

```sh
docker compose exec etcd_service_1 etcdctl put "concert:58b85029-af94-498e-ae3a-2fda2b5d6c5a:available:basic" "10000"
```

