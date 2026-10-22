package marketdata

import (
	"bearabull/internal/storage"
	"fmt"
	"log"
)

// Constituent represents a stock in the default universe.
type Constituent struct {
	Symbol      string
	Ticker      string
	CompanyName string
	Sector      string
	Exchange    string
}

// Nifty50Universe contains the verified constituents of the NIFTY 50 index.
var Nifty50Universe = []Constituent{
	{Symbol: "RELIANCE.NS", Ticker: "RELIANCE", CompanyName: "Reliance Industries Ltd", Sector: "Oil & Gas", Exchange: "NSE"},
	{Symbol: "TCS.NS", Ticker: "TCS", CompanyName: "Tata Consultancy Services Ltd", Sector: "Information Technology", Exchange: "NSE"},
	{Symbol: "HDFCBANK.NS", Ticker: "HDFCBANK", CompanyName: "HDFC Bank Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "ICICIBANK.NS", Ticker: "ICICIBANK", CompanyName: "ICICI Bank Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "BHARTIARTL.NS", Ticker: "BHARTIARTL", CompanyName: "Bharti Airtel Ltd", Sector: "Telecommunication", Exchange: "NSE"},
	{Symbol: "INFY.NS", Ticker: "INFY", CompanyName: "Infosys Ltd", Sector: "Information Technology", Exchange: "NSE"},
	{Symbol: "SBIN.NS", Ticker: "SBIN", CompanyName: "State Bank of India", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "ITC.NS", Ticker: "ITC", CompanyName: "ITC Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "HINDUNILVR.NS", Ticker: "HINDUNILVR", CompanyName: "Hindustan Unilever Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "LT.NS", Ticker: "LT", CompanyName: "Larsen & Toubro Ltd", Sector: "Construction", Exchange: "NSE"},
	{Symbol: "BAJFINANCE.NS", Ticker: "BAJFINANCE", CompanyName: "Bajaj Finance Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "HCLTECH.NS", Ticker: "HCLTECH", CompanyName: "HCL Technologies Ltd", Sector: "Information Technology", Exchange: "NSE"},
	{Symbol: "MARUTI.NS", Ticker: "MARUTI", CompanyName: "Maruti Suzuki India Ltd", Sector: "Automobile", Exchange: "NSE"},
	{Symbol: "SUNPHARMA.NS", Ticker: "SUNPHARMA", CompanyName: "Sun Pharmaceutical Industries Ltd", Sector: "Healthcare", Exchange: "NSE"},
	{Symbol: "ADANIENT.NS", Ticker: "ADANIENT", CompanyName: "Adani Enterprises Ltd", Sector: "Metals & Mining", Exchange: "NSE"},
	{Symbol: "KOTAKBANK.NS", Ticker: "KOTAKBANK", CompanyName: "Kotak Mahindra Bank Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "TITAN.NS", Ticker: "TITAN", CompanyName: "Titan Company Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "ONGC.NS", Ticker: "ONGC", CompanyName: "Oil & Natural Gas Corporation Ltd", Sector: "Oil & Gas", Exchange: "NSE"},
	{Symbol: "TATAMOTORS.NS", Ticker: "TATAMOTORS", CompanyName: "Tata Motors Ltd", Sector: "Automobile", Exchange: "NSE"},
	{Symbol: "NTPC.NS", Ticker: "NTPC", CompanyName: "NTPC Ltd", Sector: "Power", Exchange: "NSE"},
	{Symbol: "AXISBANK.NS", Ticker: "AXISBANK", CompanyName: "Axis Bank Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "ADANIPORTS.NS", Ticker: "ADANIPORTS", CompanyName: "Adani Ports & SEZ Ltd", Sector: "Services", Exchange: "NSE"},
	{Symbol: "POWERGRID.NS", Ticker: "POWERGRID", CompanyName: "Power Grid Corporation of India Ltd", Sector: "Power", Exchange: "NSE"},
	{Symbol: "ULTRACEMCO.NS", Ticker: "ULTRACEMCO", CompanyName: "UltraTech Cement Ltd", Sector: "Construction Materials", Exchange: "NSE"},
	{Symbol: "COALINDIA.NS", Ticker: "COALINDIA", CompanyName: "Coal India Ltd", Sector: "Metals & Mining", Exchange: "NSE"},
	{Symbol: "BAJAJFINSV.NS", Ticker: "BAJAJFINSV", CompanyName: "Bajaj Finserv Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "WIPRO.NS", Ticker: "WIPRO", CompanyName: "Wipro Ltd", Sector: "Information Technology", Exchange: "NSE"},
	{Symbol: "M&M.NS", Ticker: "M&M", CompanyName: "Mahindra & Mahindra Ltd", Sector: "Automobile", Exchange: "NSE"},
	{Symbol: "TATASTEEL.NS", Ticker: "TATASTEEL", CompanyName: "Tata Steel Ltd", Sector: "Metals", Exchange: "NSE"},
	{Symbol: "ASIANPAINT.NS", Ticker: "ASIANPAINT", CompanyName: "Asian Paints Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "JSWSTEEL.NS", Ticker: "JSWSTEEL", CompanyName: "JSW Steel Ltd", Sector: "Metals", Exchange: "NSE"},
	{Symbol: "GRASIM.NS", Ticker: "GRASIM", CompanyName: "Grasim Industries Ltd", Sector: "Construction Materials", Exchange: "NSE"},
	{Symbol: "TECHM.NS", Ticker: "TECHM", CompanyName: "Tech Mahindra Ltd", Sector: "Information Technology", Exchange: "NSE"},
	{Symbol: "HINDALCO.NS", Ticker: "HINDALCO", CompanyName: "Hindalco Industries Ltd", Sector: "Metals", Exchange: "NSE"},
	{Symbol: "NESTLEIND.NS", Ticker: "NESTLEIND", CompanyName: "Nestle India Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "CIPLA.NS", Ticker: "CIPLA", CompanyName: "Cipla Ltd", Sector: "Healthcare", Exchange: "NSE"},
	{Symbol: "SBILIFE.NS", Ticker: "SBILIFE", CompanyName: "SBI Life Insurance Company Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "DRREDDY.NS", Ticker: "DRREDDY", CompanyName: "Dr. Reddy's Laboratories Ltd", Sector: "Healthcare", Exchange: "NSE"},
	{Symbol: "BPCL.NS", Ticker: "BPCL", CompanyName: "Bharat Petroleum Corporation Ltd", Sector: "Oil & Gas", Exchange: "NSE"},
	{Symbol: "EICHERMOT.NS", Ticker: "EICHERMOT", CompanyName: "Eicher Motors Ltd", Sector: "Automobile", Exchange: "NSE"},
	{Symbol: "TATACONSUM.NS", Ticker: "TATACONSUM", CompanyName: "Tata Consumer Products Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "BRITANNIA.NS", Ticker: "BRITANNIA", CompanyName: "Britannia Industries Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "DIVISLAB.NS", Ticker: "DIVISLAB", CompanyName: "Divi's Laboratories Ltd", Sector: "Healthcare", Exchange: "NSE"},
	{Symbol: "APOLLOHOSP.NS", Ticker: "APOLLOHOSP", CompanyName: "Apollo Hospitals Enterprise Ltd", Sector: "Healthcare", Exchange: "NSE"},
	{Symbol: "HEROMOTOCO.NS", Ticker: "HEROMOTOCO", CompanyName: "Hero MotoCorp Ltd", Sector: "Automobile", Exchange: "NSE"},
	{Symbol: "SHREECEM.NS", Ticker: "SHREECEM", CompanyName: "Shree Cement Ltd", Sector: "Construction Materials", Exchange: "NSE"},
	{Symbol: "BEL.NS", Ticker: "BEL", CompanyName: "Bharat Electronics Ltd", Sector: "Capital Goods", Exchange: "NSE"},
	{Symbol: "TRENT.NS", Ticker: "TRENT", CompanyName: "Trent Ltd", Sector: "Consumer Goods", Exchange: "NSE"},
	{Symbol: "SHRIRAMFIN.NS", Ticker: "SHRIRAMFIN", CompanyName: "Shriram Finance Ltd", Sector: "Financial Services", Exchange: "NSE"},
	{Symbol: "INDUSINDBK.NS", Ticker: "INDUSINDBK", CompanyName: "IndusInd Bank Ltd", Sector: "Financial Services", Exchange: "NSE"},
}

// SeedUniverse checks if stocks table is populated, and if not seeds it with NIFTY 50 constituents.
func SeedUniverse(db *storage.DB) error {
	stocks, err := db.GetStocks(false)
	if err != nil {
		return fmt.Errorf("failed checking stocks table: %w", err)
	}

	if len(stocks) >= len(Nifty50Universe) {
		log.Printf("[UNIVERSE] Stock universe already seeded (%d constituents present)", len(stocks))
		return nil
	}

	log.Printf("[UNIVERSE] Seeding NIFTY 50 universe (%d constituents)...", len(Nifty50Universe))
	for _, c := range Nifty50Universe {
		s := storage.Stock{
			Symbol:      c.Symbol,
			Ticker:      c.Ticker,
			CompanyName: c.CompanyName,
			Sector:      c.Sector,
			Exchange:    c.Exchange,
			IsActive:    true,
		}
		if err := db.UpsertStock(s); err != nil {
			return fmt.Errorf("failed seeding stock %s: %w", c.Symbol, err)
		}
	}
	log.Printf("[UNIVERSE] Successfully seeded %d NIFTY 50 stocks", len(Nifty50Universe))
	return nil
}
