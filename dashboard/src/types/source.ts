// Source Manager API Types

/**
 * Error detail for a single row in an Excel import
 */
export interface ImportExcelRowError {
  row: number
  error: string
}

/**
 * Response from POST /api/v1/sources/import-excel
 */
export interface ImportExcelResult {
  created: number
  updated: number
  errors: ImportExcelRowError[]
}
