import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { firstValueFrom } from 'rxjs';

/** Base URL of the ocr2pdf API (see examples/be). */
export const API_BASE = 'http://localhost:8080';

export interface ConvertOptions {
  /** Tesseract language, e.g. `eng`, `vie`, `vie+eng`. */
  lang?: string;
  /** Page segmentation mode, 0–13. */
  psm?: number;
  /** Force a DPI; omit to let the server read it from the file. */
  dpi?: number;
  /** Minimum word confidence, 0–100. */
  conf?: number;
}

/** Thin client for the ocr2pdf HTTP API. */
@Injectable({ providedIn: 'root' })
export class OcrApi {
  private readonly http = inject(HttpClient);

  /** Uploads a file and resolves with the searchable PDF. */
  async convert(file: File, options: ConvertOptions = {}): Promise<Blob> {
    const form = new FormData();
    form.append('file', file);

    for (const [key, value] of Object.entries(options)) {
      if (value !== undefined && value !== null && value !== '') {
        form.append(key, String(value));
      }
    }

    try {
      return await firstValueFrom(
        this.http.post(`${API_BASE}/api/convert`, form, { responseType: 'blob' }),
      );
    } catch (error) {
      throw new Error(await this.errorMessage(error));
    }
  }

  /** Lists the Tesseract traineddata installed on the server. */
  async languages(): Promise<string[]> {
    try {
      const body = await firstValueFrom(
        this.http.get<{ languages: string[] }>(`${API_BASE}/api/languages`),
      );
      return body.languages ?? [];
    } catch {
      return [];
    }
  }

  /** The API answers errors as JSON, which arrives as a Blob because of `responseType: 'blob'`. */
  private async errorMessage(error: unknown): Promise<string> {
    if (error instanceof HttpErrorResponse) {
      const body = error.error;
      if (body instanceof Blob) {
        try {
          const parsed = JSON.parse(await body.text()) as { error?: string };
          if (parsed.error) {
            return parsed.error;
          }
        } catch {
          // not JSON, fall through
        }
      }
      if (error.status === 0) {
        return `Cannot reach the API at ${API_BASE}. Is examples/be running?`;
      }
      return `HTTP ${error.status}: ${error.statusText}`;
    }
    return error instanceof Error ? error.message : 'Unknown error';
  }
}
