import { Component, afterNextRender, computed, inject, signal } from '@angular/core';
import { OcrApi } from './ocr-api';
import { PdfViewer } from './pdf-viewer/pdf-viewer';

interface LanguageOption {
  code: string;
  label: string;
}

const ALL_LANGUAGES: LanguageOption[] = [
  { code: 'auto', label: 'Auto-detect' },
  { code: 'vie', label: 'Tiếng Việt' },
  { code: 'vie+eng', label: 'Vietnamese + English' },
  { code: 'eng', label: 'English' },
];

@Component({
  imports: [PdfViewer],
  selector: 'app-root',
  styleUrl: './app.css',
  templateUrl: './app.html',
})
export class App {
  private readonly api = inject(OcrApi);

  protected readonly file = signal<File | null>(null);
  protected readonly result = signal<Blob | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly languages = signal<string[]>([]);
  protected readonly lang = signal('auto');

  protected readonly fileName = computed(() => {
    const file = this.file();
    if (!file) {
      return 'document.pdf';
    }
    return `${file.name.replace(/\.[^.]+$/, '')}-searchable.pdf`;
  });

  /** Only offer languages the server actually has traineddata for. */
  protected readonly languageOptions = computed(() => {
    const installed = this.languages();
    if (installed.length === 0) {
      return ALL_LANGUAGES;
    }
    return ALL_LANGUAGES.filter(
      (option) =>
        option.code === 'auto' ||
        option.code.split('+').every((part) => installed.includes(part)),
    );
  });

  /** True when the server has no Vietnamese traineddata — copied text will lose diacritics. */
  protected readonly missingVietnamese = computed(
    () => this.languages().length > 0 && !this.languages().includes('vie'),
  );

  constructor() {
    // Browser only: SSR has no API to talk to.
    afterNextRender(() => void this.loadLanguages());
  }

  protected async onFile(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    await this.convert(input.files?.[0] ?? null);
    input.value = ''; // allow picking the same file again
  }

  protected onDragOver(event: DragEvent): void {
    event.preventDefault();
  }

  protected async onDrop(event: DragEvent): Promise<void> {
    event.preventDefault();
    await this.convert(event.dataTransfer?.files?.[0] ?? null);
  }

  protected onLanguage(event: Event): void {
    this.lang.set((event.target as HTMLSelectElement).value);
  }

  private async loadLanguages(): Promise<void> {
    this.languages.set(await this.api.languages());
  }

  private async convert(file: File | null): Promise<void> {
    if (!file) {
      return;
    }
    if (!/\.(png|jpe?g|gif|pdf)$/i.test(file.name)) {
      this.error.set('Choose a PNG, JPG, GIF or PDF file.');
      return;
    }

    this.file.set(file);
    this.result.set(null);
    this.error.set(null);
    this.busy.set(true);

    try {
      const lang = this.lang();
      this.result.set(await this.api.convert(file, lang === 'auto' ? {} : { lang }));
    } catch (error) {
      this.error.set(error instanceof Error ? error.message : String(error));
    } finally {
      this.busy.set(false);
    }
  }
}
