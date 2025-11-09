package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var sessions = sync.Map{}

type Session struct {
	ID         string            `json:"id"`
	Amount     int64             `json:"amount"`
	Currency   string            `json:"currency"`
	LineItems  []LineItem        `json:"line_items"`
	SuccessURL string            `json:"success_url"`
	CancelURL  string            `json:"cancel_url"`
	Status     string            `json:"status"`
	URL        string            `json:"url"`
	CreatedAt  time.Time         `json:"created_at"`
	Metadata   map[string]string `json:"metadata"`
}

type LineItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Quantity    int64  `json:"quantity"`
}

type CreateSessionRequest struct {
	LineItems  []LineItem        `json:"line_items"`
	SuccessURL string            `json:"success_url"`
	CancelURL  string            `json:"cancel_url"`
	Metadata   map[string]string `json:"metadata"`
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "cs_test_" + hex.EncodeToString(b)
}

func createSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionID := generateID()

	var totalAmount int64
	var currency string
	for _, item := range req.LineItems {
		totalAmount += item.Amount * item.Quantity
		currency = item.Currency
	}

	session := Session{
		ID:         sessionID,
		Amount:     totalAmount,
		Currency:   currency,
		LineItems:  req.LineItems,
		SuccessURL: req.SuccessURL,
		CancelURL:  req.CancelURL,
		Status:     "open",
		URL:        fmt.Sprintf("http://192.168.1.74:12222/checkout/%s", sessionID),
		CreatedAt:  time.Now(),
		Metadata:   req.Metadata,
	}

	sessions.Store(sessionID, session)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func checkoutPageHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/checkout/"):]

	val, ok := sessions.Load(sessionID)
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session := val.(Session)

	tmpl := `
<!DOCTYPE html>
<html>
<head>
	<title>Mock Payment Checkout</title>
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: #f6f9fc;
			padding: 20px;
		}
		.container {
			max-width: 500px;
			margin: 50px auto;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
			overflow: hidden;
		}
		.header {
			background: #635bff;
			color: white;
			padding: 20px;
			text-align: center;
		}
		.content {
			padding: 30px;
		}
		.line-item {
			display: flex;
			justify-content: space-between;
			padding: 15px 0;
			border-bottom: 1px solid #e6e6e6;
		}
		.line-item:last-child {
			border-bottom: 2px solid #635bff;
			font-weight: bold;
			font-size: 18px;
		}
		.item-details {
			flex: 1;
		}
		.item-name {
			font-weight: 500;
			margin-bottom: 5px;
		}
		.item-description {
			font-size: 14px;
			color: #666;
		}
		.item-price {
			text-align: right;
			font-weight: 500;
		}
		.card-form {
			margin: 30px 0;
		}
		.form-group {
			margin-bottom: 20px;
		}
		label {
			display: block;
			margin-bottom: 8px;
			font-weight: 500;
			color: #32325d;
		}
		input {
			width: 100%;
			padding: 12px;
			border: 1px solid #e6e6e6;
			border-radius: 4px;
			font-size: 16px;
		}
		input:focus {
			outline: none;
			border-color: #635bff;
		}
		.button-group {
			display: flex;
			gap: 10px;
			margin-top: 30px;
		}
		button {
			flex: 1;
			padding: 14px;
			border: none;
			border-radius: 4px;
			font-size: 16px;
			font-weight: 500;
			cursor: pointer;
			transition: all 0.2s;
		}
		.pay-button {
			background: #635bff;
			color: white;
		}
		.pay-button:hover {
			background: #4f46e5;
		}
		.cancel-button {
			background: #e6e6e6;
			color: #32325d;
		}
		.cancel-button:hover {
			background: #d4d4d4;
		}
		.test-badge {
			background: #ffd700;
			color: #000;
			padding: 4px 12px;
			border-radius: 12px;
			font-size: 12px;
			font-weight: bold;
			display: inline-block;
			margin-top: 10px;
		}
		.test-info {
			background: #fff3cd;
			border: 1px solid #ffeaa7;
			padding: 15px;
			border-radius: 4px;
			margin-bottom: 20px;
			font-size: 14px;
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>🔒 Secure Checkout</h1>
			<span class="test-badge">TEST MODE</span>
		</div>
		<div class="content">
			<div class="test-info">
				ℹ️ This is a mock payment processor. Use any card number (e.g., 4242 4242 4242 4242)
			</div>

			<h2 style="margin-bottom: 20px;">Order Summary</h2>
			{{range .LineItems}}
			<div class="line-item">
				<div class="item-details">
					<div class="item-name">{{.Name}}</div>
					<div class="item-description">{{.Description}}</div>
					<div class="item-description">Quantity: {{.Quantity}}</div>
				</div>
				<div class="item-price">${{formatAmount .Amount}} {{.Currency}}</div>
			</div>
			{{end}}
			<div class="line-item">
				<div class="item-details">
					<div class="item-name">Total</div>
				</div>
				<div class="item-price">${{formatAmount .Amount}} {{.Currency}}</div>
			</div>

			<form class="card-form" onsubmit="return handlePayment(event)">
				<div class="form-group">
					<label>Card Number</label>
					<input type="text" placeholder="4242 4242 4242 4242" value="4242 4242 4242 4242" required>
				</div>
				<div style="display: flex; gap: 15px;">
					<div class="form-group" style="flex: 1;">
						<label>Expiry Date</label>
						<input type="text" placeholder="MM/YY" value="12/25" required>
					</div>
					<div class="form-group" style="flex: 1;">
						<label>CVC</label>
						<input type="text" placeholder="123" value="123" required>
					</div>
				</div>
				<div class="form-group">
					<label>Cardholder Name</label>
					<input type="text" placeholder="John Doe" value="Test User" required>
				</div>

				<div class="button-group">
					<button type="button" class="cancel-button" onclick="handleCancel()">Cancel</button>
					<button type="submit" class="pay-button">Pay ${{formatAmount .Amount}}</button>
				</div>
			</form>
		</div>
	</div>

	<script>
		function handlePayment(e) {
			e.preventDefault();

			// Simulate payment processing
			const button = e.target.querySelector('.pay-button');
			button.textContent = 'Processing...';
			button.disabled = true;

			setTimeout(() => {
				fetch('/complete/{{.ID}}', { method: 'POST' })
					.then(() => {
						window.location.href = '{{.SuccessURL}}';
					});
			}, 1500);

			return false;
		}

		function handleCancel() {
			window.location.href = '{{.CancelURL}}';
		}
	</script>
</body>
</html>
`

	funcMap := template.FuncMap{
		"formatAmount": func(amount int64) string {
			return fmt.Sprintf("%.2f", float64(amount)/100)
		},
	}

	t := template.Must(template.New("checkout").Funcs(funcMap).Parse(tmpl))
	t.Execute(w, session)
}

func completePaymentHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Path[len("/complete/"):]

	val, ok := sessions.Load(sessionID)
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session := val.(Session)
	session.Status = "complete"
	sessions.Store(sessionID, session)

	w.WriteHeader(http.StatusOK)
}

func main() {
	godotenv.Load()

	http.HandleFunc("/v1/checkout/sessions", createSessionHandler)
	http.HandleFunc("/checkout/", checkoutPageHandler)
	http.HandleFunc("/complete/", completePaymentHandler)

	serverPort := os.Getenv("SERVER_PORT")

	log.Printf("Mock Payment Processor starting on :%s", serverPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", serverPort), nil))
}
