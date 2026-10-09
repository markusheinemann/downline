# Flight events

> [!Note]
> **Disclaimer on AI usage**
> This document was written with AI based on data exploration done by a human. The numbers come from queries the
> author ran and were reviewed by the author.

This document describes how Tier 2 turns the raw state vectors of Tier 1 into flight events, and why. The rules are
based on an exploration of one day of collected data. Each rule lists the observation that supports it.

## Goal

Tier 2 reconstructs each aircraft's day as a chain of flights. For every flight it records two events:

* **Departure**: the aircraft leaves its origin airport.
* **Arrival**: the aircraft reaches its destination airport.

The reactionary delay analysis compares these events with an empirical schedule. The schedule is the median time of
each repeating flight over a reference period.

## Data basis

The exploration uses one full day, 2026-10-07 (UTC). The day contains 8.4 million state vectors. To keep the analysis
focused, it includes only aircraft that used a Lufthansa callsign (`DLH...`) at least once on that day. All state
vectors of these aircraft are included, also those with a different or empty callsign.

The OpenSky Network API returns a snapshot of all aircraft every 100 seconds (the collector's poll interval). The
following fields of a state vector are used:

| Field          | Position | Unit    | Use                                     |
|----------------|----------|---------|-----------------------------------------|
| `icao24`       | 0        |         | Identifies the aircraft (transponder).  |
| `callsign`     | 1        |         | Identifies the flight.                  |
| `longitude`    | 5        | degrees | Position.                               |
| `latitude`     | 6        | degrees | Position.                               |
| `baro_altitude`| 7        | m       | Barometric altitude above sea level.    |
| `on_ground`    | 8        |         | `true` if the transponder reports ground. |
| `velocity`     | 9        | m/s     | Ground speed.                           |
| `vertical_rate`| 11       | m/s     | Climb or descent rate.                  |
| `geo_altitude` | 13       | m       | Geometric altitude.                     |

Positions are zero based, as in the OpenSky documentation. DuckDB lists are one based, so the queries below use
`s[1]` for `icao24`.

## Method

### Loading the archives

DuckDB reads the zstd compressed JSONL archives directly. Each line is one API response. The `states` array is
unnested into one row per aircraft and snapshot:

```sql
CREATE OR REPLACE TABLE states AS
SELECT r.time                        AS snapshot,
       trim(s[1], '"')               AS icao24,
       trim(trim(s[2], '"'))         AS callsign,
       s[6]::DOUBLE                  AS lon,
       s[7]::DOUBLE                  AS lat,
       s[8]::DOUBLE                  AS baro_alt,
       s[9]::BOOLEAN                 AS on_ground,
       s[10]::DOUBLE                 AS velocity,
       s[12]::DOUBLE                 AS vertical_rate,
       s[14]::DOUBLE                 AS geo_alt
FROM read_json('archive_2026-10-07_*.zst', format = 'newline_delimited', compression = 'zstd') AS r,
     unnest(r.states) AS t(s);
```

### Splitting into segments

A first, simple rule splits the observations of an aircraft into segments: a new segment starts when the aircraft
was not seen for more than 5 minutes (3 missed snapshots). Segments without any airborne point or with fewer than 5
points are dropped.

```sql
CREATE OR REPLACE TABLE flights AS
WITH dlh_aircraft AS (
    SELECT DISTINCT icao24 FROM states WHERE callsign LIKE 'DLH%'
),
points AS (
    SELECT s.icao24, s.snapshot, s.callsign, s.on_ground, s.baro_alt,
           s.snapshot - lag(s.snapshot) OVER w AS gap_before
    FROM states s JOIN dlh_aircraft USING (icao24)
    WINDOW w AS (PARTITION BY s.icao24 ORDER BY s.snapshot)
),
segmented AS (
    SELECT *,
           sum(CASE WHEN gap_before IS NULL OR gap_before > 300 THEN 1 ELSE 0 END)
               OVER (PARTITION BY icao24 ORDER BY snapshot) AS flight_no
    FROM points
),
flight_rows AS (
    SELECT icao24, flight_no,
           arg_min(on_ground, snapshot) AS starts_on_ground,
           arg_max(on_ground, snapshot) AS ends_on_ground,
           arg_min(baro_alt, snapshot)  AS first_alt,
           arg_max(baro_alt, snapshot)  AS last_alt,
           arg_min(callsign, snapshot) FILTER (WHERE callsign <> '') AS first_callsign,
           arg_max(callsign, snapshot) FILTER (WHERE callsign <> '') AS last_callsign,
           count(*)                     AS points,
           bool_or(NOT on_ground)       AS was_airborne,
           min(snapshot)                AS first_seen,
           max(snapshot)                AS last_seen
    FROM segmented
    GROUP BY icao24, flight_no
)
SELECT * FROM flight_rows WHERE was_airborne AND points >= 5;
```

The segments are then examined at their start, at their end, and at the gap to the next segment.

## Findings

### One aircraft in detail

Aircraft `3c658c` flew 10 flights on 2026-10-07. Its track shows patterns that also hold for the full data set:

* Between two flights the aircraft is not seen for 30 to 85 minutes, and the callsign changes.
* At the start of a flight, the callsign is empty for a few snapshots before the new callsign appears.
* 5 of the 10 departures are first seen on the ground. The others are first seen in the air at 400 to 1700 m.
* 9 of the 10 arrivals end on the ground with a speed of 0 to 9 m/s. One ends in the air at 495 m.

### Segment starts and ends

The 5 minute rule produces 1454 segments.

| Segment            | Count | Share |
|--------------------|------:|------:|
| Starts on ground   | 721   | 50 %  |
| Ends on ground     | 832   | 57 %  |
| Both               | 451   | 31 %  |
| Neither            | 352   | 24 %  |

### Altitude at the end of a segment

622 segments do not end on the ground. Their last altitude, rounded to the nearest 1000 m:

| Last altitude     | Count | Interpretation                                   |
|-------------------|------:|--------------------------------------------------|
| below 500 m       | 180   | Final approach, ground contact not received.     |
| 500 to 1499 m     | 41    | Approach.                                        |
| 1500 to 8499 m    | 55    | Climb or descent, unclear.                       |
| 8500 m and above  | 344   | Cruise. The aircraft left the receiver coverage. |
| unknown           | 2     | No altitude reported.                            |

About 15 % of all segments (221) end on approach. Together with the segments that end on the ground, about 72 % of
the segments end close to an airport.

Of the 344 segments that end at cruising altitude, only 23 end at the end of the day. The others end during the day
because the aircraft left the coverage.

### Continuity of reception

The API returns only aircraft that are currently received. Areas without coverage therefore produce no data at all,
and coverage gaps appear as time gaps in the track of an aircraft. Within the covered area, reception is stable: an
airborne aircraft that is seen in one snapshot is seen again in the next snapshot (gap of at most 150 seconds) in 97
to 98 % of the cases. This query uses all aircraft, not only Lufthansa.

| Region                         | Airborne points | Seen in next snapshot |
|--------------------------------|----------------:|----------------------:|
| outside Europe                 | 5,367,965       | 97.1 %                |
| Europe without the Germany box | 1,761,072       | 98.3 %                |
| Germany box                    | 319,111         | 98.2 %                |

The regions are rectangles: the Germany box covers 47.3 to 55.1° N and 5.9 to 15.0° E and includes parts of the
neighbouring countries, Europe covers 35 to 72° N and 25° W to 45° E. There is no notable difference between the
regions. The query cannot show where the covered area ends, because it only sees points that were received.

```sql
WITH points AS (
    SELECT lat, lon,
           lead(snapshot) OVER (PARTITION BY icao24 ORDER BY snapshot) - snapshot AS gap_after
    FROM states
    WHERE NOT on_ground AND lat IS NOT NULL
)
SELECT CASE WHEN lat BETWEEN 47.3 AND 55.1 AND lon BETWEEN 5.9 AND 15.0 THEN 'Germany'
            WHEN lat BETWEEN 35 AND 72 AND lon BETWEEN -25 AND 45        THEN 'rest of Europe'
            ELSE 'outside Europe' END                                     AS region,
       count(*)                                                           AS points,
       round(100 * avg(CASE WHEN gap_after <= 150 THEN 1 ELSE 0 END), 1)  AS seen_next_snapshot_pct
FROM points
WHERE gap_after IS NOT NULL
GROUP BY region
ORDER BY points DESC;
```

### What follows a segment that ends at cruising altitude

| Next segment of the same aircraft | Count | Median gap | Interpretation                                  |
|-----------------------------------|------:|-----------:|-------------------------------------------------|
| starts at cruising altitude       | 260   | 63 min     | The same flight, split by a coverage gap.       |
| starts low or at mid altitude     | 11    | 42 min     | Landing and departure were not observed.        |
| starts on the ground              | 0     |            |                                                 |
| none on the same day              | 73    |            | 23 end of day, the rest landed outside coverage. |

```sql
WITH ordered AS (
    SELECT *,
           lead(first_alt)        OVER w AS next_first_alt,
           lead(starts_on_ground) OVER w AS next_starts_on_ground,
           lead(first_seen)       OVER w AS next_first_seen
    FROM flights
    WINDOW w AS (PARTITION BY icao24 ORDER BY flight_no)
)
SELECT CASE WHEN next_first_seen IS NULL THEN 'no later segment'
            WHEN next_starts_on_ground   THEN 'next starts on ground'
            WHEN next_first_alt >= 8500  THEN 'next starts at cruise'
            ELSE 'next starts low or mid' END                  AS what_follows,
       count(*)                                                 AS segments,
       round(median(next_first_seen - last_seen) / 60)          AS median_gap_min
FROM ordered
WHERE NOT ends_on_ground AND last_alt >= 8500
GROUP BY what_follows
ORDER BY segments DESC;
```

## Rules

### Flight boundaries

A time gap alone does not separate two flights. Three quarters of the segments that end at cruising altitude
continue as the same flight after a gap of about one hour. The rule therefore combines the gap with altitude and
callsign:

* A **new flight** starts if the callsign changes from one non-empty value to a different non-empty value.
* A **new flight** starts if there is a gap of more than 5 minutes and the aircraft was low or on the ground before
  or after the gap.
* A gap at cruising altitude on both sides with the same callsign does **not** start a new flight.
* An empty callsign never counts as a change.

The callsign condition protects against a specific error: during a long gap, an aircraft can land, turn around and
take off again without being observed. Without the callsign condition, the two flights would be merged.

#### Verification

The 260 segments that end at cruising altitude and are followed by a segment that starts at cruising altitude are the
candidates for a merge. The last non-empty callsign before the gap is compared with the first non-empty callsign after
the gap:

| Callsign before and after the gap | Count | Median gap | Decision |
|-----------------------------------|------:|-----------:|----------|
| same                              | 251   | 63 min     | merge    |
| different                         | 7     | 237 min    | split    |
| missing on one side               | 2     | 8 min      | merge    |

All 7 segments with a different callsign are real flight changes:

| Aircraft | Before    | After     | Gap     |
|----------|-----------|-----------|--------:|
| `3c658e` | `DLH1558` | `DLH1559` | 93 min  |
| `3c666b` | `DLH9HE`  | `DLH5TN`  | 115 min |
| `3c64b0` | `DLH582`  | `DLH583`  | 197 min |
| `3c4a08` | `DLH752`  | `DLH753`  | 237 min |
| `3c4a21` | `DLH728`  | `DLH729`  | 267 min |
| `3c66c5` | `DLH726`  | `DLH727`  | 597 min |
| `3c6701` | `DLH732`  | `DLH733`  | 653 min |

Six of them are pairs of an outbound and a return flight: the flight number increases by one. The aircraft left the
coverage, landed at an airport outside the coverage, and returned on the next flight. A merge based on altitude alone
would have combined two flights with up to 11 hours between them.

```sql
WITH ordered AS (
    SELECT *,
           lead(first_alt)      OVER w AS next_first_alt,
           lead(first_callsign) OVER w AS next_first_callsign,
           lead(first_seen)     OVER w AS next_first_seen
    FROM flights
    WINDOW w AS (PARTITION BY icao24 ORDER BY flight_no)
)
SELECT CASE WHEN last_callsign IS NULL OR next_first_callsign IS NULL THEN 'callsign missing'
            WHEN last_callsign = next_first_callsign                  THEN 'same callsign'
            ELSE 'different callsign' END                   AS callsign_check,
       count(*)                                              AS segments,
       round(median(next_first_seen - last_seen) / 60)       AS median_gap_min
FROM ordered
WHERE NOT ends_on_ground AND last_alt >= 8500 AND next_first_alt >= 8500
GROUP BY callsign_check
ORDER BY segments DESC;
```

### Events as time windows

The API delivers a snapshot every 100 seconds, and coverage near the ground is incomplete. The exact moment of a
departure or arrival is therefore not observable. Each event is stored as a time window:

* `not_before`: the last observation before the event.
* `not_after`: the first observation after the event.

The width of the window expresses the uncertainty of the event. Analyses can filter events with a window that is too
wide.

### Airport assignment

An event is assigned to an airport with a three dimensional zone around the airport: a radius around the airport
reference point and a maximum height above the airport elevation. The airport list with positions and elevations is
provided separately.

Entering the zone alone does not count as a landing, because aircraft also pass through the zone during a go-around
or an overflight. A second signal is required, for example:

* the aircraft reports `on_ground`, or
* the segment ends inside the zone, below the zone's maximum height, while descending.

`baro_altitude` is relative to sea level. The height check must subtract the airport elevation.

## Limitations

* **Coverage.** Within the covered area reception is stable, but flights become invisible when they leave it. About
  60 flights per day of the observed aircraft have an arrival that cannot be observed: the segment ends at cruising
  altitude and the aircraft does not reappear on the same day, or reappears already at low altitude somewhere else.
  Where exactly the covered area ends was not measured.
* **Departures in the air.** Half of the departures are first seen in the air. For these, the departure window starts
  at the last observation at the origin, if there is one, and is wider than for departures seen on the ground.
* **One day, one airline.** The findings are based on one day and Lufthansa aircraft only. Other days and airlines
  can show different patterns, especially airlines with fewer flights over Germany.

## Open questions

* Which aircraft are included: Lufthansa only, or more airlines.
* How a flight is identified across days for the schedule, for example by callsign, origin and destination. Not
  every callsign contains the flight number: alphanumeric callsigns such as `DLH9HE` occur.
* Which reference period the median schedule uses. The IATA winter schedule starts on 2026-10-25, so the reference
  period must not span this date. Weekdays may need separate medians.
* How a day is defined for an aircraft (UTC day or local operating day).
* Storage format of Tier 2 (for example Parquet).
