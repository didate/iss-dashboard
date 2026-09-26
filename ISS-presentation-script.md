# ISS Dashboard — 15-minute presentation (4 slides)

> English is kept simple and short, easy to read aloud.
> For each slide: **what to put on the slide** + **what to say** (speaker script, ~3 min each).

---

## SLIDE 1 — Context: a health facility registry for Guinea

**On the slide (bullets):**
- Project: national **health map** ("carte sanitaire") of Guinea
- Funded by **Enabel** (Belgian development agency)
- Goal: collect **attribute data** on every health facility
  (equipment, staff, services, infrastructure, water & energy)
- Data is collected through **DHIS2**, the national health information system
- Challenge: a lot of data → we must **check its quality** and make it **easy to read**

**What to say:**
> "Good morning. This project is part of the national health map of Guinea, funded by Enabel.
> The goal is simple: we want to know, for every health facility in the country, what it really has —
> its equipment, its staff, the services it offers, its water and its energy.
> The survey is filled in DHIS2, our national health information system.
> But collecting is not enough. We get thousands of records, and some are incomplete or wrong.
> So we needed a tool to check the quality of this data, and to turn it into something we can read and use.
> That tool is the ISS Dashboard."

---

## SLIDE 2 — What is ISS and how it works  *(put the diagram here)*

**On the slide:**
- **ISS** = "Informations des Structures Sanitaires" (health facility information)
- A web tool that reads the survey from DHIS2 and turns it into a clear dashboard
- **Two goals:** (1) check **data quality**  ·  (2) show **how facilities are equipped** (analysis)
- Insert the diagram → `iss-dataflow.svg`
- Key idea: **all calculations happen once, in the backend.** The screen only displays → always fast.

**What to say:**
> "ISS stands for 'Informations des Structures Sanitaires'.
> Let me show you how it works with this diagram.
> On the left, DHIS2 holds the survey — one record for each facility.
> The backend pulls this data, reads the values, and runs a set of quality checks.
> Then it calculates the results — scores and summaries — and saves them in a small database.
> The web dashboard on the right only reads these ready-made results. It does no calculation, so it is always fast.
> This refresh runs automatically every few hours, and an administrator can also click 'Sync now' at any time.
> ISS has two goals: first, check the quality of the data; second, analyse how our facilities are equipped."

---

## SLIDE 3 — Data quality: finding problems automatically

**On the slide:**
- For each facility, ISS runs a set of **rules**. Examples:
  - **Missing** required fields (status, manager name, date)
  - **Impossible** values (functional equipment > total equipment)
  - **Inconsistencies** (lab declared open, but no working microscope)
  - **Outliers**, **duplicates**, **empty records**
- Each problem has a **severity**: error / warning / info
- Each facility gets a **quality score** (0–100)
- Results are grouped **by district and region** → we see where data is weak

**What to say:**
> "Let's talk about data quality first, because this is the main goal.
> For every facility, the tool applies a list of rules.
> For example: a required field is missing — like the status or the date.
> Or an impossible value — the number of working refrigerators is higher than the total number of refrigerators.
> Or an inconsistency — the facility says it has a laboratory, but it has no working microscope.
> The tool also finds outliers, duplicates, and empty records.
> Each problem gets a level: error, warning, or information.
> And each facility gets a quality score from 0 to 100.
> Finally, we group everything by district and region.
> So in one look, a manager can see which districts have weak data and need to correct it."

---

## SLIDE 4 — Usage analysis & value

**On the slide:**
- Beyond quality, ISS summarises the facilities:
  - **Census** — how many facilities, by status (public/private), district, region
  - **Services** available, and **equipment** that really works (cold chain, imaging, beds…)
  - **Human resources** by profile (doctors, nurses, midwives, community workers…)
  - **Water & energy** (WASH) coverage
- **Value:** faster and more reliable data · know what each facility has · find the gaps
- Automatic sync + one-click refresh · nothing is calculated on the user side

**What to say:**
> "The second goal is analysis.
> Beyond quality, ISS gives a clear picture of the network of facilities.
> It counts the facilities by status, by district and by region.
> It shows which services are available, and which equipment really works — for example the cold chain for vaccines, or imaging.
> It shows human resources by profile — doctors, nurses, midwives, community workers.
> And it shows access to water and energy.
> The value is simple: better and faster data, a clear view of what each facility has, and an easy way to find the gaps.
> Everything refreshes automatically, and the dashboard stays simple and fast.
> Thank you. I am happy to take your questions."

---

### Timing guide (≈15 min)
| Slide | Topic | Time |
|------|-------|------|
| 1 | Context | 3 min |
| 2 | Tool + diagram | 4 min |
| 3 | Data quality | 4 min |
| 4 | Usage + value | 3 min |
| — | Questions | 1 min |

### Notes
- "Enabel" (not "Enable") is the Belgian development agency — please double-check this is the correct funder before presenting.
- If the audience is non-technical, you can skip the words "backend / API / SQLite" on slide 2 and just say "the tool" and "the database".
