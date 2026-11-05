import { isPlatformBrowser } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  PLATFORM_ID,
  computed,
  effect,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { DomSanitizer, type SafeResourceUrl } from '@angular/platform-browser';

/** A PDF can come from a URL, a Blob or a File picked from disk. */
export type PdfSource = string | Blob | File | null;

/**
 * Renders a PDF with the browser's built-in viewer inside an iframe.
 *
 * ```html
 * <app-pdf-viewer [src]="file()" [fileName]="fileName()" height="70vh" />
 * ```
 *
 * `src` accepts an http(s) URL, a `Blob` or a `File`; object URLs are created and
 * revoked automatically, and nothing runs during server-side rendering.
 */
@Component({
  selector: 'app-pdf-viewer',
  templateUrl: './pdf-viewer.html',
  styleUrl: './pdf-viewer.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PdfViewer {
  private readonly sanitizer = inject(DomSanitizer);
  private readonly platformId = inject(PLATFORM_ID);
  private readonly frame = viewChild<ElementRef<HTMLIFrameElement>>('frame');

  /** PDF source: an http(s) URL, a Blob or a File. */
  readonly src = input<PdfSource>(null);
  /** Name shown in the toolbar and used by the download button. */
  readonly fileName = input('document.pdf');
  /** Height of the viewer area. */
  readonly height = input('75vh');

  private readonly rawUrl = signal<string | null>(null);

  protected readonly url = computed<SafeResourceUrl | null>(() => {
    const raw = this.rawUrl();
    return raw ? this.sanitizer.bypassSecurityTrustResourceUrl(raw) : null;
  });
  protected readonly loading = signal(false);

  constructor() {
    effect((onCleanup) => {
      const src = this.src();

      if (!isPlatformBrowser(this.platformId) || !src) {
        this.rawUrl.set(null);
        this.loading.set(false);
        return;
      }

      if (typeof src === 'string') {
        this.loading.set(true);
        this.rawUrl.set(src);
        return;
      }

      const objectUrl = URL.createObjectURL(src);
      this.loading.set(true);
      this.rawUrl.set(objectUrl);
      onCleanup(() => URL.revokeObjectURL(objectUrl));
    });
  }

  protected onLoad(): void {
    this.loading.set(false);
  }

  protected reload(): void {
    this.frame()?.nativeElement.contentWindow?.location.reload();
  }

  protected openInNewTab(): void {
    const raw = this.rawUrl();
    if (raw && isPlatformBrowser(this.platformId)) {
      window.open(raw, '_blank', 'noopener');
    }
  }
}
