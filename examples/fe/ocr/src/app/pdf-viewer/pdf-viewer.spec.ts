import { TestBed } from '@angular/core/testing';
import { PdfViewer } from './pdf-viewer';

describe('PdfViewer', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [PdfViewer],
    }).compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(PdfViewer);
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('should show the empty state without a source', async () => {
    const fixture = TestBed.createComponent(PdfViewer);
    await fixture.whenStable();

    const element = fixture.nativeElement as HTMLElement;
    expect(element.textContent).toContain('No PDF loaded');
    expect(element.querySelector('iframe')).toBeNull();
  });
});
