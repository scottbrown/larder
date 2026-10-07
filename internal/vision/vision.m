#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>
#include "vision.h"

// Vision downsamples very tall images, which ruins small receipt text, so
// pages are recognised in overlapping horizontal bands of at most this height.
static const size_t kBandHeight = 2400;
static const size_t kBandOverlap = 200;

static char *copy_error(NSString *msg) {
    return strdup([msg UTF8String]);
}

static CGImageRef render_page(CGPDFPageRef page, double dpi) {
    CGRect box = CGPDFPageGetBoxRect(page, kCGPDFCropBox);
    int rotation = ((CGPDFPageGetRotationAngle(page) % 360) + 360) % 360;
    double scale = dpi / 72.0;
    BOOL sideways = rotation == 90 || rotation == 270;
    size_t width = (size_t)ceil((sideways ? box.size.height : box.size.width) * scale);
    size_t height = (size_t)ceil((sideways ? box.size.width : box.size.height) * scale);

    CGColorSpaceRef space = CGColorSpaceCreateDeviceRGB();
    CGContextRef ctx = CGBitmapContextCreate(NULL, width, height, 8, 0, space,
                                             (CGBitmapInfo)kCGImageAlphaNoneSkipLast);
    CGColorSpaceRelease(space);
    if (ctx == NULL) {
        return NULL;
    }
    CGContextSetRGBFillColor(ctx, 1, 1, 1, 1);
    CGContextFillRect(ctx, CGRectMake(0, 0, width, height));
    CGContextScaleCTM(ctx, scale, scale);
    switch (rotation) {
    case 90:
        CGContextTranslateCTM(ctx, 0, box.size.width);
        CGContextRotateCTM(ctx, -M_PI_2);
        break;
    case 180:
        CGContextTranslateCTM(ctx, box.size.width, box.size.height);
        CGContextRotateCTM(ctx, M_PI);
        break;
    case 270:
        CGContextTranslateCTM(ctx, box.size.height, 0);
        CGContextRotateCTM(ctx, M_PI_2);
        break;
    }
    CGContextTranslateCTM(ctx, -box.origin.x, -box.origin.y);
    CGContextDrawPDFPage(ctx, page);
    CGImageRef image = CGBitmapContextCreateImage(ctx);
    CGContextRelease(ctx);
    return image;
}

static NSArray *recognize_image(CGImageRef image, NSError **error) {
    size_t width = CGImageGetWidth(image);
    size_t height = CGImageGetHeight(image);
    NSMutableArray *out = [NSMutableArray array];
    NSMutableSet *seen = [NSMutableSet set];

    size_t step = kBandHeight - kBandOverlap;
    for (size_t top = 0; top < height; top += step) {
        size_t bandHeight = MIN(kBandHeight, height - top);
        CGImageRef band = CGImageCreateWithImageInRect(image, CGRectMake(0, top, width, bandHeight));
        VNRecognizeTextRequest *req = [[VNRecognizeTextRequest alloc] init];
        req.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
        req.usesLanguageCorrection = NO;
        VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:band options:@{}];
        BOOL ok = [handler performRequests:@[ req ] error:error];
        CGImageRelease(band);
        if (!ok) {
            return nil;
        }
        for (VNRecognizedTextObservation *obs in req.results) {
            VNRecognizedText *best = [[obs topCandidates:1] firstObject];
            if (best == nil) {
                continue;
            }
            CGRect b = obs.boundingBox;
            double pxTop = top + (1.0 - b.origin.y - b.size.height) * bandHeight;
            double pxHeight = b.size.height * bandHeight;
            double pxMid = pxTop + pxHeight / 2;
            // Skip text whose centre falls in the overlap already covered by
            // the previous band (or the next band, which will see it fully).
            BOOL lastBand = top + bandHeight >= height;
            if (top > 0 && pxMid < top + kBandOverlap / 2) {
                continue;
            }
            if (!lastBand && pxMid >= top + bandHeight - kBandOverlap / 2) {
                continue;
            }
            NSString *key = [NSString stringWithFormat:@"%@@%.0f,%.0f", best.string, b.origin.x * width, pxTop];
            if ([seen containsObject:key]) {
                continue;
            }
            [seen addObject:key];
            double dx = (obs.topRight.x - obs.topLeft.x) * width;
            double dy = (obs.topRight.y - obs.topLeft.y) * bandHeight;
            [out addObject:@{
                @"t" : best.string,
                @"x" : @(b.origin.x),
                @"y" : @(1.0 - (pxTop + pxHeight) / height),
                @"w" : @(b.size.width),
                @"h" : @(pxHeight / height),
                @"s" : @(dx > 0 ? dy / dx : 0),
            }];
        }
    }
    return out;
}

char *larder_recognize_pdf(const char *path, double dpi, char **err) {
    @autoreleasepool {
        NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
        CGPDFDocumentRef doc = CGPDFDocumentCreateWithURL((__bridge CFURLRef)url);
        if (doc == NULL) {
            *err = copy_error(@"cannot open PDF");
            return NULL;
        }
        NSMutableArray *pages = [NSMutableArray array];
        size_t count = CGPDFDocumentGetNumberOfPages(doc);
        for (size_t i = 1; i <= count; i++) {
            CGImageRef image = render_page(CGPDFDocumentGetPage(doc, i), dpi);
            if (image == NULL) {
                CGPDFDocumentRelease(doc);
                *err = copy_error([NSString stringWithFormat:@"cannot render page %zu", i]);
                return NULL;
            }
            NSError *error = nil;
            NSArray *obs = recognize_image(image, &error);
            if (obs == nil) {
                CGPDFDocumentRelease(doc);
                CGImageRelease(image);
                *err = copy_error(error ? error.localizedDescription : @"recognition failed");
                return NULL;
            }
            [pages addObject:@{
                @"width" : @(CGImageGetWidth(image)),
                @"height" : @(CGImageGetHeight(image)),
                @"observations" : obs,
            }];
            CGImageRelease(image);
        }
        CGPDFDocumentRelease(doc);

        NSError *error = nil;
        NSData *json = [NSJSONSerialization dataWithJSONObject:pages options:0 error:&error];
        if (json == nil) {
            *err = copy_error(error.localizedDescription);
            return NULL;
        }
        char *buf = malloc(json.length + 1);
        memcpy(buf, json.bytes, json.length);
        buf[json.length] = '\0';
        return buf;
    }
}
