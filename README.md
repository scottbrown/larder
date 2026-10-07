# larder

`larder` reads scanned grocery receipts and works out which products a household buys regularly, how often it buys them, and how their prices have changed over time. It produces a self-contained HTML dashboard and has terminal commands for checking the results.

It reads the receipt images itself using macOS's built-in Vision text recognition, so it needs no OCR software, cloud service or API key.

## Requirements

- macOS (text recognition uses Apple's Vision framework through cgo)
- Go 1.25 or later
- [Task](https://taskfile.dev) for building and testing

## Building

```sh
task build      # writes .build/larder
task test       # runs tests, coverage in .test/coverage.out
```

## Receipts

Receipts are PDFs named `YYYYMMDD-<Store>-Receipt[-n].pdf`, in any directory layout under one root, for example:

```
files/2026/20260102-Costco-Receipt.pdf
files/2026/20260107-Superstore-Receipt-1.pdf
```

These are the recognised stores and the filename prefixes they use:

| Store | Prefixes |
|---|---|
| Costco | `Costco` |
| Real Canadian Superstore | `Superstore`, `SuperStore`, `RealCanadianSuperstore` |
| Wholesale Club | `WholesaleClub` |
| Walmart | `Walmart` |
| Calgary Co-op | `Coop`, `CoOp`, `CalgaryCoop` |
| Sobeys, Safeway, IGA, FreshCo | `Sobeys`, `Safeway`, `IGA`, `Freshco` |
| T&T Supermarket | `T&TSupermarket`, `T&T` |
| Save-On-Foods | `SaveOnFoods` |
| Bulk Barn | `BulkBarn` |

Files with other prefixes are ignored, including `CostcoGas`, `CostcoLiquor` and statements. If the same file appears twice, it is only counted once.

## Usage

Point `larder` at the receipts with `--receipts` or the `LARDER_RECEIPTS` environment variable:

```sh
export LARDER_RECEIPTS=~/Documents/receipts

larder scan                 # recognise every receipt once (cached)
larder audit                # how many receipts parsed cleanly, by store
larder regulars             # regular products, cadence and price change
larder dashboard -o larder.html
```

| Command | What it does |
|---|---|
| `scan` | Recognises the text of every receipt in parallel and caches it |
| `audit [-v]` | Counts receipts whose items add up to the printed subtotal, by store; `-v` lists the ones that don't |
| `regulars [-n N]` | Lists regular products with how often they are bought, when they were last bought, their status, how often they were bought on sale, and how their price has changed |
| `catalogue [-n N]` | Prints YAML stubs for regular products the catalogue doesn't cover yet |
| `dashboard [-o file]` | Writes the HTML dashboard |
| `items` | Exports every parsed line item as CSV |
| `ocr <file>...` | Prints the recognised rows of a receipt, for debugging |

These global flags apply to every command:

| Flag | Default | Purpose |
|---|---|---|
| `-r, --receipts` | `$LARDER_RECEIPTS` | Root directory of receipt PDFs |
| `-c, --catalogue` | `$LARDER_CATALOGUE`, else `~/.config/larder/catalogue.yml` | Product catalogue |
| `--cache` | `~/Library/Caches/larder` | Recognised-text cache |
| `--from`, `--to` | 2019, current year | Years of receipts to include |

## The dashboard

`larder dashboard` writes a single HTML file with no external dependencies. It follows the system's light or dark theme and contains:

- **Stat tiles:** how much the regular basket's price has changed, at shelf prices and at the prices you actually paid; how many products are bought regularly; how many are due; how much was saved on sales; and how well the receipts were read.
- **Basket price index:** two lines, both chained year over year across every regular product, weighted by spend, with the first year set to 100.
  - **Shelf prices** show what stores charge at regular price.
  - **What you paid** uses prices after sales and instant savings, so it shows how buying on sale changes your real cost increase.
- **Regulars table:** searchable, filterable by status, store and category, and sortable. Each row shows the product's cadence, when it was last bought, its status, the share of purchases made on sale, its current price, and its price change at shelf prices and at the prices actually paid.
- **Product history:**
  - a timeline of every purchase day, with sale days highlighted;
  - a chart of every unit price, where each sale purchase drops from the shelf price to a ring at the price actually paid, with step lines for each year's average shelf price and average price paid;
  - how often the product was bought on sale, the total saved and the typical sale discount.
- **Biggest price changes:** the largest rises and falls among products still bought, by shelf price or by price paid.
- **Receipt quality:** reconciliation results by store.

## How it works

1. **Recognition.** Each page is rendered with CoreGraphics and read with Vision. Long receipts are read in overlapping bands, because Vision downsamples tall images. Results are cached by file content in `~/Library/Caches/larder`.
2. **Layout.** Recognised text is grouped into printed rows. Vision reports the slope of each text line, so skewed scans are straightened before grouping.
3. **Parsing.** Store-specific parsers read the items, quantities, weights, multi-buys, instant savings, voids, deposits and fees.
4. **Reconciliation.** Each receipt's items and fees are compared with its printed subtotal. If they don't match:
   - The receipt is read again at 300 and 150 DPI, since faint thermal print often reads correctly at one resolution but not another.
   - Each page of a multi-page scan is tried on its own.
   - A single unreadable price is inferred from the subtotal.
5. **Products.** An item is identified by the store's item number, UPC or PLU. Codes that are rare and one character away from a common code with the same description are treated as OCR misreads and merged. Items with the same description at the same store are merged when their prices are similar, because stores renumber products. Different pack sizes that share a description are kept apart.
6. **Sales.** An item counts as bought on sale when the receipt shows a reduction:

   | Store | What the receipt shows |
   |---|---|
   | Costco | Instant savings (`TPD/…`) |
   | Superstore | A regular price beside the price charged (`$1.49 lmt 4, $1.99 ea`), or a multi-buy |
   | Walmart | Multi-buy discounts, matched to the items they cover by their `MULTI nn` tags |
   | Sobeys, Safeway, IGA, FreshCo, Co-op | `YOU SAVED` lines |
   | Save-On-Foods | Card savings |
   | Bulk Barn | Percentage discounts |
   | T&T | A `(SALE)` marker |

   T&T doesn't print the regular price, so its sale items count toward the price paid but are left out of shelf prices.
7. **Analysis.**
   - A product is **regular** when it was bought on at least four separate days spanning at least 60 days.
   - **Cadence** is the median number of days between purchases.
   - A product's **status** compares the days since it was last bought with its cadence: stocked, due, overdue, or lapsed after three times its usual interval.
   - **Price change** is measured two ways, comparing the earliest and latest years that each have at least two purchases.
     - **Shelf price** is the quantity-weighted average regular price: what the store charges.
     - **Paid price** is the quantity-weighted average after sales and instant savings: what you actually spent per unit.
     - Both use the same purchases and the same averaging, so with no sales they agree. Any gap between them comes entirely from how often, and how deeply, you bought on sale.
   - Prices more than twice, or less than half, a product's median are treated as misreads and left out of the trends.

Flashfood orders are not included. They are app screenshots of clearance-priced items, and including them would distort price trends.

## The catalogue

The catalogue is a YAML file you edit to name products, assign categories, group the same product across stores or item numbers, and hide items:

```yaml
products:
  - name: Milk, homogenised 4 L
    category: Dairy & eggs
    match:
      - costco:457
      - superstore:6570010026
  - name: Bananas
    category: Produce
    match:
      - costco:30669
      - coop:BANANAS        # stores without codes match on description
ignore:
  - walmart:7874252086
```

Each product's history in the dashboard shows the `store:code` values it matches. `larder catalogue` prints stubs for uncatalogued regular products, ready to edit and paste in.

## Limitations

- About three-quarters of receipts reconcile exactly. Items from receipts that don't reconcile are still counted, but those receipts may be missing a line or have a misread price.
- Costco.ca order emails and pharmacy receipts carry no grocery items.
- Stores that print no item codes (Co-op, Sobeys, Safeway, T&T, Save-On-Foods, Bulk Barn) identify products by description, so OCR variation in a description can split one product in two. Use the catalogue to merge them.
