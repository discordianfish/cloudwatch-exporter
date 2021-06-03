# Experimental Cloudwatch Exporter Bulk
This exporter is intended for bulk import of cloudwatch metrics.

## Usage

   localhost:9106/metrics/<Namespace>/[<MetricName>][?opt=val]

The exporter by default listens on port 9106 and returns all cloudwatch metrics
for the given Namespace and MetricName. If MetricName is omitted, return all
metrics for the Namespace.

*Example:*

    curl localhost:9106/metrics/AWS/EC2/NetworkIn

Additional url parameters are supported:
 - stat: Statistics to retrieve, values can include Sum, SampleCount, Minimum,
   Maximum, Average.
 - delay: The newest data to request. Used to avoid collecting data that has not
   fully converged. Defaults to 600s.
 - range: How far back to request data for. Useful for cases such as Billing
   metrics that are only set every few hours. Defaults to 600s.
 - period: Period to request the metric for. Only the most recent data point is
   used. Defaults to 60s.
