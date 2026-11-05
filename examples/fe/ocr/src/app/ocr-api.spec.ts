import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { API_BASE, OcrApi } from './ocr-api';

describe('OcrApi', () => {
  let api: OcrApi;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(OcrApi);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('posts the file as multipart and returns the PDF blob', async () => {
    const file = new File(['image'], 'scan.png', { type: 'image/png' });

    const promise = api.convert(file, { lang: 'vie' });
    const request = http.expectOne(`${API_BASE}/api/convert`);

    expect(request.request.method).toBe('POST');
    const body = request.request.body as FormData;
    expect(body.get('file')).toBeInstanceOf(File);
    expect(body.get('lang')).toBe('vie');

    request.flush(new Blob(['%PDF-1.7'], { type: 'application/pdf' }));

    await expect(promise).resolves.toBeInstanceOf(Blob);
  });

  it('surfaces the JSON error message from the API', async () => {
    const file = new File(['x'], 'scan.txt', { type: 'text/plain' });

    const promise = api.convert(file);
    http
      .expectOne(`${API_BASE}/api/convert`)
      .flush(new Blob([JSON.stringify({ error: 'unsupported file type ".txt"' })]), {
        status: 415,
        statusText: 'Unsupported Media Type',
      });

    await expect(promise).rejects.toThrow('unsupported file type');
  });
});
