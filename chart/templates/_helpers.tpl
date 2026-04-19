{{/*
  Expand the name of the chart.
*/}}
{{- define "hh-resume-auto-boost.name" -}}
  {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
  Create a default fully qualified app name.
*/}}
{{- define "hh-resume-auto-boost.fullname" -}}
  {{- if .Values.fullnameOverride }}
    {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
  {{- else }}
    {{- $name := default .Chart.Name .Values.nameOverride }}
    {{- if contains $name .Release.Name }}
      {{- .Release.Name | trunc 63 | trimSuffix "-" }}
    {{- else }}
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
  Create chart name and version as used by the chart label.
*/}}
{{- define "hh-resume-auto-boost.chart" -}}
  {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
  Common labels.
*/}}
{{- define "hh-resume-auto-boost.labels" -}}
helm.sh/chart: {{ include "hh-resume-auto-boost.chart" . }}
{{ include "hh-resume-auto-boost.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/component: {{ .Values.componentOverride | default "hh-resume-auto-boost" | quote }}
{{- end }}

{{/*
  Selector labels.
*/}}
{{- define "hh-resume-auto-boost.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hh-resume-auto-boost.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
  Name of the credentials Secret.
  Returns the existingSecret name, or, if it is not set, the chart-managed one.
*/}}
{{- define "hh-resume-auto-boost.credentialsSecret" -}}
  {{- if .Values.credentials.existingSecret }}
    {{- .Values.credentials.existingSecret }}
  {{- else }}
    {{- include "hh-resume-auto-boost.fullname" . }}-credentials
  {{- end }}
{{- end }}
