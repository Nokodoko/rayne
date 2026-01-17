---
title: Logs Analysis
modified: 2026-01-13T14:12:55.873Z
tags: []
metadata: {}
time:
  live_span: 1h
template_variables: []
---


Notebooks now include advanced analysis features. You can use SQL, chain queries, and visualize results in notebooks or dashboards. 

If you've used **"Log Workspaces"** in Datadog Log Management product, notebooks now has the same features.

# Defining data source cells

With analysis features, you can define a data source cell like below

<!--Some widget snapshots failed to export, so we’ve included their JSON definitions instead. If you reimport this markdown file to Datadog, widgets will display as expected.-->

```dd-widget
{
  "id": "g42f3gxd",
  "type": "notebook_cells",
  "attributes": {
    "definition": {
      "query": {
        "data_source": "logs",
        "storage": "hot",
        "name": "datasource_1",
        "columns": [
          {
            "column": "timestamp",
            "type": "timestamp"
          },
          {
            "column": "host",
            "type": "string"
          },
          {
            "column": "service",
            "type": "string"
          },
          {
            "column": "message",
            "type": "string"
          }
        ],
        "indexes": []
      },
      "type": "analysis_data_source"
    }
  }
}
```

👉 Try it by typing `/datasource` in your notebook, or by clicking any of the data source buttons in the menus.

# Chaining queries, using SQL and transformation cells

Then query it using SQL

<!--Some widget snapshots failed to export, so we’ve included their JSON definitions instead. If you reimport this markdown file to Datadog, widgets will display as expected.-->

```dd-widget
{
  "id": "ol99cqxg",
  "type": "notebook_cells",
  "attributes": {
    "definition": {
      "query": {
        "data_source": "analysis_dataset",
        "name": "analysis_1",
        "query": {
          "type": "sql_analysis",
          "sql_query": "SELECT * FROM datasource_1"
        }
      },
      "type": "analysis_sql"
    }
  }
}
```

and / or apply pre defined transformations like join, parsing or filters

<!--Some widget snapshots failed to export, so we’ve included their JSON definitions instead. If you reimport this markdown file to Datadog, widgets will display as expected.-->

```dd-widget
{
  "id": "i7d2kgvu",
  "type": "notebook_cells",
  "attributes": {
    "definition": {
      "query": {
        "data_source": "analysis_dataset",
        "name": "transformation_1",
        "query": {
          "type": "structured_analysis",
          "source_dataset": "analysis_1",
          "transformations": []
        }
      },
      "type": "analysis_transformation"
    }
  }
}
```

# Visualization and saving to dashboards

and lastly visualize it and / or save it to a dashboard

<!--Some widget snapshots failed to export, so we’ve included their JSON definitions instead. If you reimport this markdown file to Datadog, widgets will display as expected.-->

```dd-widget
{
  "id": "dtr504al",
  "type": "notebook_cells",
  "attributes": {
    "definition": {
      "type": "toplist",
      "requests": [
        {
          "request_type": "local_dataset",
          "response_format": "tabular",
          "query": {
            "type": "structured_analysis",
            "source_dataset": "transformation_1",
            "transformations": [
              {
                "type": "filter",
                "filter": ""
              },
              {
                "type": "aggregation",
                "compute": [
                  {
                    "aggregation": "count",
                    "column": ""
                  }
                ],
                "group_by": []
              }
            ]
          }
        }
      ]
    },
    "time": null,
    "split_by": {
      "keys": [],
      "tags": []
    }
  }
}
```