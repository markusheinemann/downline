# Collector

Key considerations for the Collector.

## Scope

The collector is responsible for collecting ADSB data from the opensky network api. Under the hood, it calls the
`/stats/all` endpoint which returns
all [state vectors](https://openskynetwork.github.io/opensky-api/index.html#state-vectors) of the last 60 minutes for
all tracked movements with a resolution of 5 seconds.

## Considerations

### Rate Limiting

Each call for the `/stats/all` endpoints costs 4 credits. A standard account is limited to 4000 credits per day. Given a
day has 86,400 seconds, that means

$$\Delta t = 86400 \frac{c}{C}$$

where

- $\Delta t$ is the time between calls
- $c$ is the number of credits per call
- $C$ is the number of credits per day.

It's a good idea not to burn all credits every day and leave a reserve $r$ for retries, development work and so on.

$$\Delta t =\frac{86400 c}{C (1-r)}$$

Given we have 4000 credits per day and want to reserve 10% of the credits for retries, the time between calls should be

$$\Delta t =\frac{86400 \cdot 4}{4000 \cdot (1-0.1)} = 96 \text{ seconds between calls}$$

## Storage

During tests the responses from the `/stats/all` endpoint have an average size of 2 mb per response with 13,000 state
vectors. If 96 seconds between calls is considered:

- 900 requests per day
- 11,700,000 vectors per day
- 1.8 GB per day
- 12.6 GB per week
- 54 GB per month
- **648 GB per year**

To overcome the storage limitations, a three-tire storage solution is implemented.

### Tier 1 / Archive

The raw response data is compressed and stored in hourly archives.

### Tier 2 / Events

### Tier 3 / Derived 



