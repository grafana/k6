//line plugins/parsers/influx/machine.go.rl:1
package protocol

import (
	"errors"
	"io"
)

var (
	ErrNameParse      = errors.New("expected measurement name")
	ErrFieldParse     = errors.New("expected field")
	ErrTagParse       = errors.New("expected tag")
	ErrTimestampParse = errors.New("expected timestamp")
	ErrParse          = errors.New("parse error")
	//lint:ignore ST1012 not needed
	EOF = errors.New("EOF")
)

//line plugins/parsers/influx/machine.go.rl:310

//line plugins/parsers/influx/machine.go:25
const LineProtocol_start int = 270
const LineProtocol_first_final int = 270
const LineProtocol_error int = 0

const LineProtocol_en_main int = 270
const LineProtocol_en_discard_line int = 258
const LineProtocol_en_align int = 740
const LineProtocol_en_series int = 261

//line plugins/parsers/influx/machine.go.rl:313

type Handler interface {
	SetMeasurement(name []byte) error
	AddTag(key []byte, value []byte) error
	AddInt(key []byte, value []byte) error
	AddUint(key []byte, value []byte) error
	AddFloat(key []byte, value []byte) error
	AddString(key []byte, value []byte) error
	AddBool(key []byte, value []byte) error
	SetTimestamp(tm []byte) error
}

type machine struct {
	data         []byte
	cs           int
	p, pe, eof   int
	pb           int
	lineno       int
	sol          int
	handler      Handler
	initState    int
	key          []byte
	beginMetric  bool
	finishMetric bool
}

func NewMachine(handler Handler) *machine {
	m := &machine{
		handler:   handler,
		initState: LineProtocol_en_align,
	}

//line plugins/parsers/influx/machine.go.rl:346

//line plugins/parsers/influx/machine.go.rl:347

//line plugins/parsers/influx/machine.go.rl:348

//line plugins/parsers/influx/machine.go.rl:349

//line plugins/parsers/influx/machine.go.rl:350

//line plugins/parsers/influx/machine.go.rl:351

//line plugins/parsers/influx/machine.go:82
	{
		(m.cs) = LineProtocol_start
	}

//line plugins/parsers/influx/machine.go.rl:352

	return m
}

func NewSeriesMachine(handler Handler) *machine {
	m := &machine{
		handler:   handler,
		initState: LineProtocol_en_series,
	}

//line plugins/parsers/influx/machine.go.rl:363

//line plugins/parsers/influx/machine.go.rl:364

//line plugins/parsers/influx/machine.go.rl:365

//line plugins/parsers/influx/machine.go.rl:366

//line plugins/parsers/influx/machine.go.rl:367

//line plugins/parsers/influx/machine.go:109
	{
		(m.cs) = LineProtocol_start
	}

//line plugins/parsers/influx/machine.go.rl:368

	return m
}

func (m *machine) SetData(data []byte) {
	m.data = data
	m.p = 0
	m.pb = 0
	m.lineno = 1
	m.sol = 0
	m.pe = len(data)
	m.eof = len(data)
	m.key = nil
	m.beginMetric = false
	m.finishMetric = false

//line plugins/parsers/influx/machine.go:132
	{
		(m.cs) = LineProtocol_start
	}

//line plugins/parsers/influx/machine.go.rl:385
	m.cs = m.initState
}

// Next parses the next metric line and returns nil if it was successfully
// processed.  If the line contains a syntax error an error is returned,
// otherwise if the end of file is reached before finding a metric line then
// EOF is returned.
func (m *machine) Next() error {
	if m.p == m.pe && m.pe == m.eof {
		return EOF
	}

	m.key = nil
	m.beginMetric = false
	m.finishMetric = false

	return m.exec()
}

func (m *machine) exec() error {
	var err error

//line plugins/parsers/influx/machine.go:160
	{
		if (m.p) == (m.pe) {
			goto _test_eof
		}
		goto _resume

	_again:
		switch m.cs {
		case 270:
			goto st270
		case 1:
			goto st1
		case 2:
			goto st2
		case 3:
			goto st3
		case 0:
			goto st0
		case 4:
			goto st4
		case 5:
			goto st5
		case 6:
			goto st6
		case 7:
			goto st7
		case 271:
			goto st271
		case 272:
			goto st272
		case 273:
			goto st273
		case 8:
			goto st8
		case 9:
			goto st9
		case 10:
			goto st10
		case 11:
			goto st11
		case 12:
			goto st12
		case 13:
			goto st13
		case 14:
			goto st14
		case 15:
			goto st15
		case 16:
			goto st16
		case 17:
			goto st17
		case 18:
			goto st18
		case 19:
			goto st19
		case 20:
			goto st20
		case 21:
			goto st21
		case 22:
			goto st22
		case 23:
			goto st23
		case 24:
			goto st24
		case 25:
			goto st25
		case 26:
			goto st26
		case 27:
			goto st27
		case 28:
			goto st28
		case 29:
			goto st29
		case 30:
			goto st30
		case 31:
			goto st31
		case 32:
			goto st32
		case 274:
			goto st274
		case 275:
			goto st275
		case 33:
			goto st33
		case 34:
			goto st34
		case 276:
			goto st276
		case 277:
			goto st277
		case 278:
			goto st278
		case 35:
			goto st35
		case 279:
			goto st279
		case 280:
			goto st280
		case 281:
			goto st281
		case 282:
			goto st282
		case 283:
			goto st283
		case 284:
			goto st284
		case 285:
			goto st285
		case 286:
			goto st286
		case 287:
			goto st287
		case 288:
			goto st288
		case 289:
			goto st289
		case 290:
			goto st290
		case 291:
			goto st291
		case 292:
			goto st292
		case 293:
			goto st293
		case 294:
			goto st294
		case 295:
			goto st295
		case 296:
			goto st296
		case 36:
			goto st36
		case 37:
			goto st37
		case 297:
			goto st297
		case 298:
			goto st298
		case 299:
			goto st299
		case 38:
			goto st38
		case 39:
			goto st39
		case 40:
			goto st40
		case 41:
			goto st41
		case 42:
			goto st42
		case 300:
			goto st300
		case 301:
			goto st301
		case 302:
			goto st302
		case 303:
			goto st303
		case 43:
			goto st43
		case 304:
			goto st304
		case 305:
			goto st305
		case 306:
			goto st306
		case 307:
			goto st307
		case 308:
			goto st308
		case 309:
			goto st309
		case 310:
			goto st310
		case 311:
			goto st311
		case 312:
			goto st312
		case 313:
			goto st313
		case 314:
			goto st314
		case 315:
			goto st315
		case 316:
			goto st316
		case 317:
			goto st317
		case 318:
			goto st318
		case 319:
			goto st319
		case 320:
			goto st320
		case 321:
			goto st321
		case 322:
			goto st322
		case 323:
			goto st323
		case 324:
			goto st324
		case 325:
			goto st325
		case 44:
			goto st44
		case 45:
			goto st45
		case 46:
			goto st46
		case 47:
			goto st47
		case 48:
			goto st48
		case 49:
			goto st49
		case 50:
			goto st50
		case 51:
			goto st51
		case 52:
			goto st52
		case 53:
			goto st53
		case 326:
			goto st326
		case 327:
			goto st327
		case 328:
			goto st328
		case 54:
			goto st54
		case 55:
			goto st55
		case 56:
			goto st56
		case 57:
			goto st57
		case 58:
			goto st58
		case 59:
			goto st59
		case 329:
			goto st329
		case 330:
			goto st330
		case 60:
			goto st60
		case 331:
			goto st331
		case 332:
			goto st332
		case 333:
			goto st333
		case 334:
			goto st334
		case 335:
			goto st335
		case 336:
			goto st336
		case 337:
			goto st337
		case 338:
			goto st338
		case 339:
			goto st339
		case 340:
			goto st340
		case 341:
			goto st341
		case 342:
			goto st342
		case 343:
			goto st343
		case 344:
			goto st344
		case 345:
			goto st345
		case 346:
			goto st346
		case 347:
			goto st347
		case 348:
			goto st348
		case 349:
			goto st349
		case 350:
			goto st350
		case 61:
			goto st61
		case 351:
			goto st351
		case 352:
			goto st352
		case 353:
			goto st353
		case 62:
			goto st62
		case 354:
			goto st354
		case 355:
			goto st355
		case 356:
			goto st356
		case 357:
			goto st357
		case 358:
			goto st358
		case 359:
			goto st359
		case 360:
			goto st360
		case 361:
			goto st361
		case 362:
			goto st362
		case 363:
			goto st363
		case 364:
			goto st364
		case 365:
			goto st365
		case 366:
			goto st366
		case 367:
			goto st367
		case 368:
			goto st368
		case 369:
			goto st369
		case 370:
			goto st370
		case 371:
			goto st371
		case 372:
			goto st372
		case 373:
			goto st373
		case 63:
			goto st63
		case 64:
			goto st64
		case 65:
			goto st65
		case 66:
			goto st66
		case 67:
			goto st67
		case 374:
			goto st374
		case 68:
			goto st68
		case 69:
			goto st69
		case 70:
			goto st70
		case 71:
			goto st71
		case 72:
			goto st72
		case 375:
			goto st375
		case 376:
			goto st376
		case 377:
			goto st377
		case 73:
			goto st73
		case 74:
			goto st74
		case 378:
			goto st378
		case 379:
			goto st379
		case 75:
			goto st75
		case 380:
			goto st380
		case 76:
			goto st76
		case 381:
			goto st381
		case 382:
			goto st382
		case 383:
			goto st383
		case 384:
			goto st384
		case 385:
			goto st385
		case 386:
			goto st386
		case 387:
			goto st387
		case 388:
			goto st388
		case 389:
			goto st389
		case 390:
			goto st390
		case 391:
			goto st391
		case 392:
			goto st392
		case 393:
			goto st393
		case 394:
			goto st394
		case 395:
			goto st395
		case 396:
			goto st396
		case 397:
			goto st397
		case 398:
			goto st398
		case 399:
			goto st399
		case 400:
			goto st400
		case 77:
			goto st77
		case 78:
			goto st78
		case 79:
			goto st79
		case 80:
			goto st80
		case 81:
			goto st81
		case 82:
			goto st82
		case 83:
			goto st83
		case 84:
			goto st84
		case 85:
			goto st85
		case 86:
			goto st86
		case 87:
			goto st87
		case 88:
			goto st88
		case 89:
			goto st89
		case 90:
			goto st90
		case 401:
			goto st401
		case 402:
			goto st402
		case 403:
			goto st403
		case 404:
			goto st404
		case 91:
			goto st91
		case 92:
			goto st92
		case 93:
			goto st93
		case 94:
			goto st94
		case 405:
			goto st405
		case 406:
			goto st406
		case 95:
			goto st95
		case 96:
			goto st96
		case 407:
			goto st407
		case 97:
			goto st97
		case 98:
			goto st98
		case 408:
			goto st408
		case 409:
			goto st409
		case 99:
			goto st99
		case 410:
			goto st410
		case 411:
			goto st411
		case 100:
			goto st100
		case 101:
			goto st101
		case 412:
			goto st412
		case 413:
			goto st413
		case 414:
			goto st414
		case 415:
			goto st415
		case 416:
			goto st416
		case 417:
			goto st417
		case 418:
			goto st418
		case 419:
			goto st419
		case 420:
			goto st420
		case 421:
			goto st421
		case 422:
			goto st422
		case 423:
			goto st423
		case 424:
			goto st424
		case 425:
			goto st425
		case 426:
			goto st426
		case 427:
			goto st427
		case 428:
			goto st428
		case 429:
			goto st429
		case 102:
			goto st102
		case 430:
			goto st430
		case 431:
			goto st431
		case 432:
			goto st432
		case 103:
			goto st103
		case 104:
			goto st104
		case 433:
			goto st433
		case 434:
			goto st434
		case 435:
			goto st435
		case 105:
			goto st105
		case 436:
			goto st436
		case 437:
			goto st437
		case 438:
			goto st438
		case 439:
			goto st439
		case 440:
			goto st440
		case 441:
			goto st441
		case 442:
			goto st442
		case 443:
			goto st443
		case 444:
			goto st444
		case 445:
			goto st445
		case 446:
			goto st446
		case 447:
			goto st447
		case 448:
			goto st448
		case 449:
			goto st449
		case 450:
			goto st450
		case 451:
			goto st451
		case 452:
			goto st452
		case 453:
			goto st453
		case 454:
			goto st454
		case 455:
			goto st455
		case 106:
			goto st106
		case 456:
			goto st456
		case 457:
			goto st457
		case 458:
			goto st458
		case 459:
			goto st459
		case 460:
			goto st460
		case 461:
			goto st461
		case 462:
			goto st462
		case 463:
			goto st463
		case 464:
			goto st464
		case 465:
			goto st465
		case 466:
			goto st466
		case 467:
			goto st467
		case 468:
			goto st468
		case 469:
			goto st469
		case 470:
			goto st470
		case 471:
			goto st471
		case 472:
			goto st472
		case 473:
			goto st473
		case 474:
			goto st474
		case 475:
			goto st475
		case 476:
			goto st476
		case 477:
			goto st477
		case 107:
			goto st107
		case 108:
			goto st108
		case 109:
			goto st109
		case 110:
			goto st110
		case 111:
			goto st111
		case 478:
			goto st478
		case 112:
			goto st112
		case 479:
			goto st479
		case 480:
			goto st480
		case 113:
			goto st113
		case 481:
			goto st481
		case 482:
			goto st482
		case 483:
			goto st483
		case 484:
			goto st484
		case 485:
			goto st485
		case 486:
			goto st486
		case 487:
			goto st487
		case 488:
			goto st488
		case 489:
			goto st489
		case 114:
			goto st114
		case 115:
			goto st115
		case 116:
			goto st116
		case 490:
			goto st490
		case 117:
			goto st117
		case 118:
			goto st118
		case 119:
			goto st119
		case 491:
			goto st491
		case 120:
			goto st120
		case 121:
			goto st121
		case 492:
			goto st492
		case 493:
			goto st493
		case 122:
			goto st122
		case 123:
			goto st123
		case 124:
			goto st124
		case 125:
			goto st125
		case 494:
			goto st494
		case 495:
			goto st495
		case 496:
			goto st496
		case 126:
			goto st126
		case 497:
			goto st497
		case 498:
			goto st498
		case 499:
			goto st499
		case 500:
			goto st500
		case 501:
			goto st501
		case 502:
			goto st502
		case 503:
			goto st503
		case 504:
			goto st504
		case 505:
			goto st505
		case 506:
			goto st506
		case 507:
			goto st507
		case 508:
			goto st508
		case 509:
			goto st509
		case 510:
			goto st510
		case 511:
			goto st511
		case 512:
			goto st512
		case 513:
			goto st513
		case 514:
			goto st514
		case 515:
			goto st515
		case 516:
			goto st516
		case 127:
			goto st127
		case 128:
			goto st128
		case 517:
			goto st517
		case 518:
			goto st518
		case 519:
			goto st519
		case 520:
			goto st520
		case 521:
			goto st521
		case 522:
			goto st522
		case 523:
			goto st523
		case 524:
			goto st524
		case 525:
			goto st525
		case 129:
			goto st129
		case 130:
			goto st130
		case 131:
			goto st131
		case 526:
			goto st526
		case 132:
			goto st132
		case 133:
			goto st133
		case 134:
			goto st134
		case 527:
			goto st527
		case 135:
			goto st135
		case 136:
			goto st136
		case 528:
			goto st528
		case 529:
			goto st529
		case 137:
			goto st137
		case 138:
			goto st138
		case 139:
			goto st139
		case 530:
			goto st530
		case 531:
			goto st531
		case 140:
			goto st140
		case 532:
			goto st532
		case 141:
			goto st141
		case 533:
			goto st533
		case 534:
			goto st534
		case 535:
			goto st535
		case 536:
			goto st536
		case 537:
			goto st537
		case 538:
			goto st538
		case 539:
			goto st539
		case 540:
			goto st540
		case 142:
			goto st142
		case 143:
			goto st143
		case 144:
			goto st144
		case 541:
			goto st541
		case 145:
			goto st145
		case 146:
			goto st146
		case 147:
			goto st147
		case 542:
			goto st542
		case 148:
			goto st148
		case 149:
			goto st149
		case 543:
			goto st543
		case 544:
			goto st544
		case 545:
			goto st545
		case 546:
			goto st546
		case 547:
			goto st547
		case 548:
			goto st548
		case 549:
			goto st549
		case 550:
			goto st550
		case 551:
			goto st551
		case 552:
			goto st552
		case 553:
			goto st553
		case 554:
			goto st554
		case 555:
			goto st555
		case 556:
			goto st556
		case 557:
			goto st557
		case 558:
			goto st558
		case 559:
			goto st559
		case 560:
			goto st560
		case 561:
			goto st561
		case 562:
			goto st562
		case 150:
			goto st150
		case 151:
			goto st151
		case 563:
			goto st563
		case 564:
			goto st564
		case 565:
			goto st565
		case 152:
			goto st152
		case 566:
			goto st566
		case 567:
			goto st567
		case 153:
			goto st153
		case 568:
			goto st568
		case 569:
			goto st569
		case 570:
			goto st570
		case 571:
			goto st571
		case 572:
			goto st572
		case 573:
			goto st573
		case 574:
			goto st574
		case 575:
			goto st575
		case 576:
			goto st576
		case 577:
			goto st577
		case 578:
			goto st578
		case 579:
			goto st579
		case 580:
			goto st580
		case 581:
			goto st581
		case 582:
			goto st582
		case 583:
			goto st583
		case 584:
			goto st584
		case 585:
			goto st585
		case 154:
			goto st154
		case 155:
			goto st155
		case 586:
			goto st586
		case 156:
			goto st156
		case 587:
			goto st587
		case 588:
			goto st588
		case 589:
			goto st589
		case 590:
			goto st590
		case 591:
			goto st591
		case 592:
			goto st592
		case 593:
			goto st593
		case 594:
			goto st594
		case 157:
			goto st157
		case 158:
			goto st158
		case 159:
			goto st159
		case 595:
			goto st595
		case 160:
			goto st160
		case 161:
			goto st161
		case 162:
			goto st162
		case 596:
			goto st596
		case 163:
			goto st163
		case 164:
			goto st164
		case 597:
			goto st597
		case 598:
			goto st598
		case 165:
			goto st165
		case 166:
			goto st166
		case 167:
			goto st167
		case 168:
			goto st168
		case 169:
			goto st169
		case 170:
			goto st170
		case 599:
			goto st599
		case 600:
			goto st600
		case 601:
			goto st601
		case 602:
			goto st602
		case 603:
			goto st603
		case 604:
			goto st604
		case 605:
			goto st605
		case 606:
			goto st606
		case 607:
			goto st607
		case 608:
			goto st608
		case 609:
			goto st609
		case 610:
			goto st610
		case 611:
			goto st611
		case 612:
			goto st612
		case 613:
			goto st613
		case 614:
			goto st614
		case 615:
			goto st615
		case 616:
			goto st616
		case 617:
			goto st617
		case 171:
			goto st171
		case 172:
			goto st172
		case 173:
			goto st173
		case 618:
			goto st618
		case 619:
			goto st619
		case 620:
			goto st620
		case 174:
			goto st174
		case 621:
			goto st621
		case 622:
			goto st622
		case 175:
			goto st175
		case 623:
			goto st623
		case 624:
			goto st624
		case 625:
			goto st625
		case 626:
			goto st626
		case 627:
			goto st627
		case 176:
			goto st176
		case 177:
			goto st177
		case 178:
			goto st178
		case 628:
			goto st628
		case 179:
			goto st179
		case 180:
			goto st180
		case 181:
			goto st181
		case 629:
			goto st629
		case 182:
			goto st182
		case 183:
			goto st183
		case 630:
			goto st630
		case 631:
			goto st631
		case 184:
			goto st184
		case 632:
			goto st632
		case 633:
			goto st633
		case 634:
			goto st634
		case 185:
			goto st185
		case 186:
			goto st186
		case 187:
			goto st187
		case 635:
			goto st635
		case 188:
			goto st188
		case 189:
			goto st189
		case 190:
			goto st190
		case 636:
			goto st636
		case 191:
			goto st191
		case 192:
			goto st192
		case 637:
			goto st637
		case 638:
			goto st638
		case 193:
			goto st193
		case 194:
			goto st194
		case 195:
			goto st195
		case 639:
			goto st639
		case 196:
			goto st196
		case 197:
			goto st197
		case 640:
			goto st640
		case 641:
			goto st641
		case 642:
			goto st642
		case 643:
			goto st643
		case 644:
			goto st644
		case 645:
			goto st645
		case 646:
			goto st646
		case 647:
			goto st647
		case 198:
			goto st198
		case 199:
			goto st199
		case 200:
			goto st200
		case 648:
			goto st648
		case 201:
			goto st201
		case 202:
			goto st202
		case 203:
			goto st203
		case 649:
			goto st649
		case 204:
			goto st204
		case 205:
			goto st205
		case 650:
			goto st650
		case 651:
			goto st651
		case 206:
			goto st206
		case 207:
			goto st207
		case 208:
			goto st208
		case 652:
			goto st652
		case 653:
			goto st653
		case 654:
			goto st654
		case 655:
			goto st655
		case 656:
			goto st656
		case 657:
			goto st657
		case 658:
			goto st658
		case 659:
			goto st659
		case 660:
			goto st660
		case 661:
			goto st661
		case 662:
			goto st662
		case 663:
			goto st663
		case 664:
			goto st664
		case 665:
			goto st665
		case 666:
			goto st666
		case 667:
			goto st667
		case 668:
			goto st668
		case 669:
			goto st669
		case 670:
			goto st670
		case 209:
			goto st209
		case 210:
			goto st210
		case 211:
			goto st211
		case 212:
			goto st212
		case 213:
			goto st213
		case 671:
			goto st671
		case 214:
			goto st214
		case 215:
			goto st215
		case 672:
			goto st672
		case 673:
			goto st673
		case 674:
			goto st674
		case 675:
			goto st675
		case 676:
			goto st676
		case 677:
			goto st677
		case 678:
			goto st678
		case 679:
			goto st679
		case 680:
			goto st680
		case 216:
			goto st216
		case 217:
			goto st217
		case 218:
			goto st218
		case 681:
			goto st681
		case 219:
			goto st219
		case 220:
			goto st220
		case 221:
			goto st221
		case 682:
			goto st682
		case 222:
			goto st222
		case 223:
			goto st223
		case 683:
			goto st683
		case 684:
			goto st684
		case 224:
			goto st224
		case 225:
			goto st225
		case 226:
			goto st226
		case 685:
			goto st685
		case 227:
			goto st227
		case 228:
			goto st228
		case 686:
			goto st686
		case 687:
			goto st687
		case 688:
			goto st688
		case 689:
			goto st689
		case 690:
			goto st690
		case 691:
			goto st691
		case 692:
			goto st692
		case 693:
			goto st693
		case 229:
			goto st229
		case 230:
			goto st230
		case 231:
			goto st231
		case 694:
			goto st694
		case 232:
			goto st232
		case 233:
			goto st233
		case 695:
			goto st695
		case 696:
			goto st696
		case 697:
			goto st697
		case 698:
			goto st698
		case 699:
			goto st699
		case 700:
			goto st700
		case 701:
			goto st701
		case 702:
			goto st702
		case 234:
			goto st234
		case 235:
			goto st235
		case 236:
			goto st236
		case 703:
			goto st703
		case 237:
			goto st237
		case 238:
			goto st238
		case 239:
			goto st239
		case 704:
			goto st704
		case 240:
			goto st240
		case 241:
			goto st241
		case 705:
			goto st705
		case 706:
			goto st706
		case 242:
			goto st242
		case 243:
			goto st243
		case 244:
			goto st244
		case 707:
			goto st707
		case 708:
			goto st708
		case 709:
			goto st709
		case 710:
			goto st710
		case 711:
			goto st711
		case 712:
			goto st712
		case 713:
			goto st713
		case 714:
			goto st714
		case 715:
			goto st715
		case 716:
			goto st716
		case 717:
			goto st717
		case 718:
			goto st718
		case 719:
			goto st719
		case 720:
			goto st720
		case 721:
			goto st721
		case 722:
			goto st722
		case 723:
			goto st723
		case 724:
			goto st724
		case 725:
			goto st725
		case 245:
			goto st245
		case 246:
			goto st246
		case 726:
			goto st726
		case 247:
			goto st247
		case 248:
			goto st248
		case 727:
			goto st727
		case 728:
			goto st728
		case 729:
			goto st729
		case 730:
			goto st730
		case 731:
			goto st731
		case 732:
			goto st732
		case 733:
			goto st733
		case 734:
			goto st734
		case 249:
			goto st249
		case 250:
			goto st250
		case 251:
			goto st251
		case 735:
			goto st735
		case 252:
			goto st252
		case 253:
			goto st253
		case 254:
			goto st254
		case 736:
			goto st736
		case 255:
			goto st255
		case 256:
			goto st256
		case 737:
			goto st737
		case 738:
			goto st738
		case 257:
			goto st257
		case 258:
			goto st258
		case 739:
			goto st739
		case 261:
			goto st261
		case 741:
			goto st741
		case 742:
			goto st742
		case 262:
			goto st262
		case 263:
			goto st263
		case 264:
			goto st264
		case 265:
			goto st265
		case 743:
			goto st743
		case 266:
			goto st266
		case 744:
			goto st744
		case 267:
			goto st267
		case 268:
			goto st268
		case 269:
			goto st269
		case 740:
			goto st740
		case 259:
			goto st259
		case 260:
			goto st260
		}

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof
		}
	_resume:
		switch m.cs {
		case 270:
			goto st_case_270
		case 1:
			goto st_case_1
		case 2:
			goto st_case_2
		case 3:
			goto st_case_3
		case 0:
			goto st_case_0
		case 4:
			goto st_case_4
		case 5:
			goto st_case_5
		case 6:
			goto st_case_6
		case 7:
			goto st_case_7
		case 271:
			goto st_case_271
		case 272:
			goto st_case_272
		case 273:
			goto st_case_273
		case 8:
			goto st_case_8
		case 9:
			goto st_case_9
		case 10:
			goto st_case_10
		case 11:
			goto st_case_11
		case 12:
			goto st_case_12
		case 13:
			goto st_case_13
		case 14:
			goto st_case_14
		case 15:
			goto st_case_15
		case 16:
			goto st_case_16
		case 17:
			goto st_case_17
		case 18:
			goto st_case_18
		case 19:
			goto st_case_19
		case 20:
			goto st_case_20
		case 21:
			goto st_case_21
		case 22:
			goto st_case_22
		case 23:
			goto st_case_23
		case 24:
			goto st_case_24
		case 25:
			goto st_case_25
		case 26:
			goto st_case_26
		case 27:
			goto st_case_27
		case 28:
			goto st_case_28
		case 29:
			goto st_case_29
		case 30:
			goto st_case_30
		case 31:
			goto st_case_31
		case 32:
			goto st_case_32
		case 274:
			goto st_case_274
		case 275:
			goto st_case_275
		case 33:
			goto st_case_33
		case 34:
			goto st_case_34
		case 276:
			goto st_case_276
		case 277:
			goto st_case_277
		case 278:
			goto st_case_278
		case 35:
			goto st_case_35
		case 279:
			goto st_case_279
		case 280:
			goto st_case_280
		case 281:
			goto st_case_281
		case 282:
			goto st_case_282
		case 283:
			goto st_case_283
		case 284:
			goto st_case_284
		case 285:
			goto st_case_285
		case 286:
			goto st_case_286
		case 287:
			goto st_case_287
		case 288:
			goto st_case_288
		case 289:
			goto st_case_289
		case 290:
			goto st_case_290
		case 291:
			goto st_case_291
		case 292:
			goto st_case_292
		case 293:
			goto st_case_293
		case 294:
			goto st_case_294
		case 295:
			goto st_case_295
		case 296:
			goto st_case_296
		case 36:
			goto st_case_36
		case 37:
			goto st_case_37
		case 297:
			goto st_case_297
		case 298:
			goto st_case_298
		case 299:
			goto st_case_299
		case 38:
			goto st_case_38
		case 39:
			goto st_case_39
		case 40:
			goto st_case_40
		case 41:
			goto st_case_41
		case 42:
			goto st_case_42
		case 300:
			goto st_case_300
		case 301:
			goto st_case_301
		case 302:
			goto st_case_302
		case 303:
			goto st_case_303
		case 43:
			goto st_case_43
		case 304:
			goto st_case_304
		case 305:
			goto st_case_305
		case 306:
			goto st_case_306
		case 307:
			goto st_case_307
		case 308:
			goto st_case_308
		case 309:
			goto st_case_309
		case 310:
			goto st_case_310
		case 311:
			goto st_case_311
		case 312:
			goto st_case_312
		case 313:
			goto st_case_313
		case 314:
			goto st_case_314
		case 315:
			goto st_case_315
		case 316:
			goto st_case_316
		case 317:
			goto st_case_317
		case 318:
			goto st_case_318
		case 319:
			goto st_case_319
		case 320:
			goto st_case_320
		case 321:
			goto st_case_321
		case 322:
			goto st_case_322
		case 323:
			goto st_case_323
		case 324:
			goto st_case_324
		case 325:
			goto st_case_325
		case 44:
			goto st_case_44
		case 45:
			goto st_case_45
		case 46:
			goto st_case_46
		case 47:
			goto st_case_47
		case 48:
			goto st_case_48
		case 49:
			goto st_case_49
		case 50:
			goto st_case_50
		case 51:
			goto st_case_51
		case 52:
			goto st_case_52
		case 53:
			goto st_case_53
		case 326:
			goto st_case_326
		case 327:
			goto st_case_327
		case 328:
			goto st_case_328
		case 54:
			goto st_case_54
		case 55:
			goto st_case_55
		case 56:
			goto st_case_56
		case 57:
			goto st_case_57
		case 58:
			goto st_case_58
		case 59:
			goto st_case_59
		case 329:
			goto st_case_329
		case 330:
			goto st_case_330
		case 60:
			goto st_case_60
		case 331:
			goto st_case_331
		case 332:
			goto st_case_332
		case 333:
			goto st_case_333
		case 334:
			goto st_case_334
		case 335:
			goto st_case_335
		case 336:
			goto st_case_336
		case 337:
			goto st_case_337
		case 338:
			goto st_case_338
		case 339:
			goto st_case_339
		case 340:
			goto st_case_340
		case 341:
			goto st_case_341
		case 342:
			goto st_case_342
		case 343:
			goto st_case_343
		case 344:
			goto st_case_344
		case 345:
			goto st_case_345
		case 346:
			goto st_case_346
		case 347:
			goto st_case_347
		case 348:
			goto st_case_348
		case 349:
			goto st_case_349
		case 350:
			goto st_case_350
		case 61:
			goto st_case_61
		case 351:
			goto st_case_351
		case 352:
			goto st_case_352
		case 353:
			goto st_case_353
		case 62:
			goto st_case_62
		case 354:
			goto st_case_354
		case 355:
			goto st_case_355
		case 356:
			goto st_case_356
		case 357:
			goto st_case_357
		case 358:
			goto st_case_358
		case 359:
			goto st_case_359
		case 360:
			goto st_case_360
		case 361:
			goto st_case_361
		case 362:
			goto st_case_362
		case 363:
			goto st_case_363
		case 364:
			goto st_case_364
		case 365:
			goto st_case_365
		case 366:
			goto st_case_366
		case 367:
			goto st_case_367
		case 368:
			goto st_case_368
		case 369:
			goto st_case_369
		case 370:
			goto st_case_370
		case 371:
			goto st_case_371
		case 372:
			goto st_case_372
		case 373:
			goto st_case_373
		case 63:
			goto st_case_63
		case 64:
			goto st_case_64
		case 65:
			goto st_case_65
		case 66:
			goto st_case_66
		case 67:
			goto st_case_67
		case 374:
			goto st_case_374
		case 68:
			goto st_case_68
		case 69:
			goto st_case_69
		case 70:
			goto st_case_70
		case 71:
			goto st_case_71
		case 72:
			goto st_case_72
		case 375:
			goto st_case_375
		case 376:
			goto st_case_376
		case 377:
			goto st_case_377
		case 73:
			goto st_case_73
		case 74:
			goto st_case_74
		case 378:
			goto st_case_378
		case 379:
			goto st_case_379
		case 75:
			goto st_case_75
		case 380:
			goto st_case_380
		case 76:
			goto st_case_76
		case 381:
			goto st_case_381
		case 382:
			goto st_case_382
		case 383:
			goto st_case_383
		case 384:
			goto st_case_384
		case 385:
			goto st_case_385
		case 386:
			goto st_case_386
		case 387:
			goto st_case_387
		case 388:
			goto st_case_388
		case 389:
			goto st_case_389
		case 390:
			goto st_case_390
		case 391:
			goto st_case_391
		case 392:
			goto st_case_392
		case 393:
			goto st_case_393
		case 394:
			goto st_case_394
		case 395:
			goto st_case_395
		case 396:
			goto st_case_396
		case 397:
			goto st_case_397
		case 398:
			goto st_case_398
		case 399:
			goto st_case_399
		case 400:
			goto st_case_400
		case 77:
			goto st_case_77
		case 78:
			goto st_case_78
		case 79:
			goto st_case_79
		case 80:
			goto st_case_80
		case 81:
			goto st_case_81
		case 82:
			goto st_case_82
		case 83:
			goto st_case_83
		case 84:
			goto st_case_84
		case 85:
			goto st_case_85
		case 86:
			goto st_case_86
		case 87:
			goto st_case_87
		case 88:
			goto st_case_88
		case 89:
			goto st_case_89
		case 90:
			goto st_case_90
		case 401:
			goto st_case_401
		case 402:
			goto st_case_402
		case 403:
			goto st_case_403
		case 404:
			goto st_case_404
		case 91:
			goto st_case_91
		case 92:
			goto st_case_92
		case 93:
			goto st_case_93
		case 94:
			goto st_case_94
		case 405:
			goto st_case_405
		case 406:
			goto st_case_406
		case 95:
			goto st_case_95
		case 96:
			goto st_case_96
		case 407:
			goto st_case_407
		case 97:
			goto st_case_97
		case 98:
			goto st_case_98
		case 408:
			goto st_case_408
		case 409:
			goto st_case_409
		case 99:
			goto st_case_99
		case 410:
			goto st_case_410
		case 411:
			goto st_case_411
		case 100:
			goto st_case_100
		case 101:
			goto st_case_101
		case 412:
			goto st_case_412
		case 413:
			goto st_case_413
		case 414:
			goto st_case_414
		case 415:
			goto st_case_415
		case 416:
			goto st_case_416
		case 417:
			goto st_case_417
		case 418:
			goto st_case_418
		case 419:
			goto st_case_419
		case 420:
			goto st_case_420
		case 421:
			goto st_case_421
		case 422:
			goto st_case_422
		case 423:
			goto st_case_423
		case 424:
			goto st_case_424
		case 425:
			goto st_case_425
		case 426:
			goto st_case_426
		case 427:
			goto st_case_427
		case 428:
			goto st_case_428
		case 429:
			goto st_case_429
		case 102:
			goto st_case_102
		case 430:
			goto st_case_430
		case 431:
			goto st_case_431
		case 432:
			goto st_case_432
		case 103:
			goto st_case_103
		case 104:
			goto st_case_104
		case 433:
			goto st_case_433
		case 434:
			goto st_case_434
		case 435:
			goto st_case_435
		case 105:
			goto st_case_105
		case 436:
			goto st_case_436
		case 437:
			goto st_case_437
		case 438:
			goto st_case_438
		case 439:
			goto st_case_439
		case 440:
			goto st_case_440
		case 441:
			goto st_case_441
		case 442:
			goto st_case_442
		case 443:
			goto st_case_443
		case 444:
			goto st_case_444
		case 445:
			goto st_case_445
		case 446:
			goto st_case_446
		case 447:
			goto st_case_447
		case 448:
			goto st_case_448
		case 449:
			goto st_case_449
		case 450:
			goto st_case_450
		case 451:
			goto st_case_451
		case 452:
			goto st_case_452
		case 453:
			goto st_case_453
		case 454:
			goto st_case_454
		case 455:
			goto st_case_455
		case 106:
			goto st_case_106
		case 456:
			goto st_case_456
		case 457:
			goto st_case_457
		case 458:
			goto st_case_458
		case 459:
			goto st_case_459
		case 460:
			goto st_case_460
		case 461:
			goto st_case_461
		case 462:
			goto st_case_462
		case 463:
			goto st_case_463
		case 464:
			goto st_case_464
		case 465:
			goto st_case_465
		case 466:
			goto st_case_466
		case 467:
			goto st_case_467
		case 468:
			goto st_case_468
		case 469:
			goto st_case_469
		case 470:
			goto st_case_470
		case 471:
			goto st_case_471
		case 472:
			goto st_case_472
		case 473:
			goto st_case_473
		case 474:
			goto st_case_474
		case 475:
			goto st_case_475
		case 476:
			goto st_case_476
		case 477:
			goto st_case_477
		case 107:
			goto st_case_107
		case 108:
			goto st_case_108
		case 109:
			goto st_case_109
		case 110:
			goto st_case_110
		case 111:
			goto st_case_111
		case 478:
			goto st_case_478
		case 112:
			goto st_case_112
		case 479:
			goto st_case_479
		case 480:
			goto st_case_480
		case 113:
			goto st_case_113
		case 481:
			goto st_case_481
		case 482:
			goto st_case_482
		case 483:
			goto st_case_483
		case 484:
			goto st_case_484
		case 485:
			goto st_case_485
		case 486:
			goto st_case_486
		case 487:
			goto st_case_487
		case 488:
			goto st_case_488
		case 489:
			goto st_case_489
		case 114:
			goto st_case_114
		case 115:
			goto st_case_115
		case 116:
			goto st_case_116
		case 490:
			goto st_case_490
		case 117:
			goto st_case_117
		case 118:
			goto st_case_118
		case 119:
			goto st_case_119
		case 491:
			goto st_case_491
		case 120:
			goto st_case_120
		case 121:
			goto st_case_121
		case 492:
			goto st_case_492
		case 493:
			goto st_case_493
		case 122:
			goto st_case_122
		case 123:
			goto st_case_123
		case 124:
			goto st_case_124
		case 125:
			goto st_case_125
		case 494:
			goto st_case_494
		case 495:
			goto st_case_495
		case 496:
			goto st_case_496
		case 126:
			goto st_case_126
		case 497:
			goto st_case_497
		case 498:
			goto st_case_498
		case 499:
			goto st_case_499
		case 500:
			goto st_case_500
		case 501:
			goto st_case_501
		case 502:
			goto st_case_502
		case 503:
			goto st_case_503
		case 504:
			goto st_case_504
		case 505:
			goto st_case_505
		case 506:
			goto st_case_506
		case 507:
			goto st_case_507
		case 508:
			goto st_case_508
		case 509:
			goto st_case_509
		case 510:
			goto st_case_510
		case 511:
			goto st_case_511
		case 512:
			goto st_case_512
		case 513:
			goto st_case_513
		case 514:
			goto st_case_514
		case 515:
			goto st_case_515
		case 516:
			goto st_case_516
		case 127:
			goto st_case_127
		case 128:
			goto st_case_128
		case 517:
			goto st_case_517
		case 518:
			goto st_case_518
		case 519:
			goto st_case_519
		case 520:
			goto st_case_520
		case 521:
			goto st_case_521
		case 522:
			goto st_case_522
		case 523:
			goto st_case_523
		case 524:
			goto st_case_524
		case 525:
			goto st_case_525
		case 129:
			goto st_case_129
		case 130:
			goto st_case_130
		case 131:
			goto st_case_131
		case 526:
			goto st_case_526
		case 132:
			goto st_case_132
		case 133:
			goto st_case_133
		case 134:
			goto st_case_134
		case 527:
			goto st_case_527
		case 135:
			goto st_case_135
		case 136:
			goto st_case_136
		case 528:
			goto st_case_528
		case 529:
			goto st_case_529
		case 137:
			goto st_case_137
		case 138:
			goto st_case_138
		case 139:
			goto st_case_139
		case 530:
			goto st_case_530
		case 531:
			goto st_case_531
		case 140:
			goto st_case_140
		case 532:
			goto st_case_532
		case 141:
			goto st_case_141
		case 533:
			goto st_case_533
		case 534:
			goto st_case_534
		case 535:
			goto st_case_535
		case 536:
			goto st_case_536
		case 537:
			goto st_case_537
		case 538:
			goto st_case_538
		case 539:
			goto st_case_539
		case 540:
			goto st_case_540
		case 142:
			goto st_case_142
		case 143:
			goto st_case_143
		case 144:
			goto st_case_144
		case 541:
			goto st_case_541
		case 145:
			goto st_case_145
		case 146:
			goto st_case_146
		case 147:
			goto st_case_147
		case 542:
			goto st_case_542
		case 148:
			goto st_case_148
		case 149:
			goto st_case_149
		case 543:
			goto st_case_543
		case 544:
			goto st_case_544
		case 545:
			goto st_case_545
		case 546:
			goto st_case_546
		case 547:
			goto st_case_547
		case 548:
			goto st_case_548
		case 549:
			goto st_case_549
		case 550:
			goto st_case_550
		case 551:
			goto st_case_551
		case 552:
			goto st_case_552
		case 553:
			goto st_case_553
		case 554:
			goto st_case_554
		case 555:
			goto st_case_555
		case 556:
			goto st_case_556
		case 557:
			goto st_case_557
		case 558:
			goto st_case_558
		case 559:
			goto st_case_559
		case 560:
			goto st_case_560
		case 561:
			goto st_case_561
		case 562:
			goto st_case_562
		case 150:
			goto st_case_150
		case 151:
			goto st_case_151
		case 563:
			goto st_case_563
		case 564:
			goto st_case_564
		case 565:
			goto st_case_565
		case 152:
			goto st_case_152
		case 566:
			goto st_case_566
		case 567:
			goto st_case_567
		case 153:
			goto st_case_153
		case 568:
			goto st_case_568
		case 569:
			goto st_case_569
		case 570:
			goto st_case_570
		case 571:
			goto st_case_571
		case 572:
			goto st_case_572
		case 573:
			goto st_case_573
		case 574:
			goto st_case_574
		case 575:
			goto st_case_575
		case 576:
			goto st_case_576
		case 577:
			goto st_case_577
		case 578:
			goto st_case_578
		case 579:
			goto st_case_579
		case 580:
			goto st_case_580
		case 581:
			goto st_case_581
		case 582:
			goto st_case_582
		case 583:
			goto st_case_583
		case 584:
			goto st_case_584
		case 585:
			goto st_case_585
		case 154:
			goto st_case_154
		case 155:
			goto st_case_155
		case 586:
			goto st_case_586
		case 156:
			goto st_case_156
		case 587:
			goto st_case_587
		case 588:
			goto st_case_588
		case 589:
			goto st_case_589
		case 590:
			goto st_case_590
		case 591:
			goto st_case_591
		case 592:
			goto st_case_592
		case 593:
			goto st_case_593
		case 594:
			goto st_case_594
		case 157:
			goto st_case_157
		case 158:
			goto st_case_158
		case 159:
			goto st_case_159
		case 595:
			goto st_case_595
		case 160:
			goto st_case_160
		case 161:
			goto st_case_161
		case 162:
			goto st_case_162
		case 596:
			goto st_case_596
		case 163:
			goto st_case_163
		case 164:
			goto st_case_164
		case 597:
			goto st_case_597
		case 598:
			goto st_case_598
		case 165:
			goto st_case_165
		case 166:
			goto st_case_166
		case 167:
			goto st_case_167
		case 168:
			goto st_case_168
		case 169:
			goto st_case_169
		case 170:
			goto st_case_170
		case 599:
			goto st_case_599
		case 600:
			goto st_case_600
		case 601:
			goto st_case_601
		case 602:
			goto st_case_602
		case 603:
			goto st_case_603
		case 604:
			goto st_case_604
		case 605:
			goto st_case_605
		case 606:
			goto st_case_606
		case 607:
			goto st_case_607
		case 608:
			goto st_case_608
		case 609:
			goto st_case_609
		case 610:
			goto st_case_610
		case 611:
			goto st_case_611
		case 612:
			goto st_case_612
		case 613:
			goto st_case_613
		case 614:
			goto st_case_614
		case 615:
			goto st_case_615
		case 616:
			goto st_case_616
		case 617:
			goto st_case_617
		case 171:
			goto st_case_171
		case 172:
			goto st_case_172
		case 173:
			goto st_case_173
		case 618:
			goto st_case_618
		case 619:
			goto st_case_619
		case 620:
			goto st_case_620
		case 174:
			goto st_case_174
		case 621:
			goto st_case_621
		case 622:
			goto st_case_622
		case 175:
			goto st_case_175
		case 623:
			goto st_case_623
		case 624:
			goto st_case_624
		case 625:
			goto st_case_625
		case 626:
			goto st_case_626
		case 627:
			goto st_case_627
		case 176:
			goto st_case_176
		case 177:
			goto st_case_177
		case 178:
			goto st_case_178
		case 628:
			goto st_case_628
		case 179:
			goto st_case_179
		case 180:
			goto st_case_180
		case 181:
			goto st_case_181
		case 629:
			goto st_case_629
		case 182:
			goto st_case_182
		case 183:
			goto st_case_183
		case 630:
			goto st_case_630
		case 631:
			goto st_case_631
		case 184:
			goto st_case_184
		case 632:
			goto st_case_632
		case 633:
			goto st_case_633
		case 634:
			goto st_case_634
		case 185:
			goto st_case_185
		case 186:
			goto st_case_186
		case 187:
			goto st_case_187
		case 635:
			goto st_case_635
		case 188:
			goto st_case_188
		case 189:
			goto st_case_189
		case 190:
			goto st_case_190
		case 636:
			goto st_case_636
		case 191:
			goto st_case_191
		case 192:
			goto st_case_192
		case 637:
			goto st_case_637
		case 638:
			goto st_case_638
		case 193:
			goto st_case_193
		case 194:
			goto st_case_194
		case 195:
			goto st_case_195
		case 639:
			goto st_case_639
		case 196:
			goto st_case_196
		case 197:
			goto st_case_197
		case 640:
			goto st_case_640
		case 641:
			goto st_case_641
		case 642:
			goto st_case_642
		case 643:
			goto st_case_643
		case 644:
			goto st_case_644
		case 645:
			goto st_case_645
		case 646:
			goto st_case_646
		case 647:
			goto st_case_647
		case 198:
			goto st_case_198
		case 199:
			goto st_case_199
		case 200:
			goto st_case_200
		case 648:
			goto st_case_648
		case 201:
			goto st_case_201
		case 202:
			goto st_case_202
		case 203:
			goto st_case_203
		case 649:
			goto st_case_649
		case 204:
			goto st_case_204
		case 205:
			goto st_case_205
		case 650:
			goto st_case_650
		case 651:
			goto st_case_651
		case 206:
			goto st_case_206
		case 207:
			goto st_case_207
		case 208:
			goto st_case_208
		case 652:
			goto st_case_652
		case 653:
			goto st_case_653
		case 654:
			goto st_case_654
		case 655:
			goto st_case_655
		case 656:
			goto st_case_656
		case 657:
			goto st_case_657
		case 658:
			goto st_case_658
		case 659:
			goto st_case_659
		case 660:
			goto st_case_660
		case 661:
			goto st_case_661
		case 662:
			goto st_case_662
		case 663:
			goto st_case_663
		case 664:
			goto st_case_664
		case 665:
			goto st_case_665
		case 666:
			goto st_case_666
		case 667:
			goto st_case_667
		case 668:
			goto st_case_668
		case 669:
			goto st_case_669
		case 670:
			goto st_case_670
		case 209:
			goto st_case_209
		case 210:
			goto st_case_210
		case 211:
			goto st_case_211
		case 212:
			goto st_case_212
		case 213:
			goto st_case_213
		case 671:
			goto st_case_671
		case 214:
			goto st_case_214
		case 215:
			goto st_case_215
		case 672:
			goto st_case_672
		case 673:
			goto st_case_673
		case 674:
			goto st_case_674
		case 675:
			goto st_case_675
		case 676:
			goto st_case_676
		case 677:
			goto st_case_677
		case 678:
			goto st_case_678
		case 679:
			goto st_case_679
		case 680:
			goto st_case_680
		case 216:
			goto st_case_216
		case 217:
			goto st_case_217
		case 218:
			goto st_case_218
		case 681:
			goto st_case_681
		case 219:
			goto st_case_219
		case 220:
			goto st_case_220
		case 221:
			goto st_case_221
		case 682:
			goto st_case_682
		case 222:
			goto st_case_222
		case 223:
			goto st_case_223
		case 683:
			goto st_case_683
		case 684:
			goto st_case_684
		case 224:
			goto st_case_224
		case 225:
			goto st_case_225
		case 226:
			goto st_case_226
		case 685:
			goto st_case_685
		case 227:
			goto st_case_227
		case 228:
			goto st_case_228
		case 686:
			goto st_case_686
		case 687:
			goto st_case_687
		case 688:
			goto st_case_688
		case 689:
			goto st_case_689
		case 690:
			goto st_case_690
		case 691:
			goto st_case_691
		case 692:
			goto st_case_692
		case 693:
			goto st_case_693
		case 229:
			goto st_case_229
		case 230:
			goto st_case_230
		case 231:
			goto st_case_231
		case 694:
			goto st_case_694
		case 232:
			goto st_case_232
		case 233:
			goto st_case_233
		case 695:
			goto st_case_695
		case 696:
			goto st_case_696
		case 697:
			goto st_case_697
		case 698:
			goto st_case_698
		case 699:
			goto st_case_699
		case 700:
			goto st_case_700
		case 701:
			goto st_case_701
		case 702:
			goto st_case_702
		case 234:
			goto st_case_234
		case 235:
			goto st_case_235
		case 236:
			goto st_case_236
		case 703:
			goto st_case_703
		case 237:
			goto st_case_237
		case 238:
			goto st_case_238
		case 239:
			goto st_case_239
		case 704:
			goto st_case_704
		case 240:
			goto st_case_240
		case 241:
			goto st_case_241
		case 705:
			goto st_case_705
		case 706:
			goto st_case_706
		case 242:
			goto st_case_242
		case 243:
			goto st_case_243
		case 244:
			goto st_case_244
		case 707:
			goto st_case_707
		case 708:
			goto st_case_708
		case 709:
			goto st_case_709
		case 710:
			goto st_case_710
		case 711:
			goto st_case_711
		case 712:
			goto st_case_712
		case 713:
			goto st_case_713
		case 714:
			goto st_case_714
		case 715:
			goto st_case_715
		case 716:
			goto st_case_716
		case 717:
			goto st_case_717
		case 718:
			goto st_case_718
		case 719:
			goto st_case_719
		case 720:
			goto st_case_720
		case 721:
			goto st_case_721
		case 722:
			goto st_case_722
		case 723:
			goto st_case_723
		case 724:
			goto st_case_724
		case 725:
			goto st_case_725
		case 245:
			goto st_case_245
		case 246:
			goto st_case_246
		case 726:
			goto st_case_726
		case 247:
			goto st_case_247
		case 248:
			goto st_case_248
		case 727:
			goto st_case_727
		case 728:
			goto st_case_728
		case 729:
			goto st_case_729
		case 730:
			goto st_case_730
		case 731:
			goto st_case_731
		case 732:
			goto st_case_732
		case 733:
			goto st_case_733
		case 734:
			goto st_case_734
		case 249:
			goto st_case_249
		case 250:
			goto st_case_250
		case 251:
			goto st_case_251
		case 735:
			goto st_case_735
		case 252:
			goto st_case_252
		case 253:
			goto st_case_253
		case 254:
			goto st_case_254
		case 736:
			goto st_case_736
		case 255:
			goto st_case_255
		case 256:
			goto st_case_256
		case 737:
			goto st_case_737
		case 738:
			goto st_case_738
		case 257:
			goto st_case_257
		case 258:
			goto st_case_258
		case 739:
			goto st_case_739
		case 261:
			goto st_case_261
		case 741:
			goto st_case_741
		case 742:
			goto st_case_742
		case 262:
			goto st_case_262
		case 263:
			goto st_case_263
		case 264:
			goto st_case_264
		case 265:
			goto st_case_265
		case 743:
			goto st_case_743
		case 266:
			goto st_case_266
		case 744:
			goto st_case_744
		case 267:
			goto st_case_267
		case 268:
			goto st_case_268
		case 269:
			goto st_case_269
		case 740:
			goto st_case_740
		case 259:
			goto st_case_259
		case 260:
			goto st_case_260
		}
		goto st_out
	st270:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof270
		}
	st_case_270:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr35
		case 11:
			goto tr459
		case 13:
			goto tr35
		case 32:
			goto tr458
		case 35:
			goto tr35
		case 44:
			goto tr35
		case 92:
			goto tr460
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr458
		}
		goto tr457
	tr33:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st1
	tr457:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st1
	st1:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof1
		}
	st_case_1:
//line plugins/parsers/influx/machine.go:3204
		switch (m.data)[(m.p)] {
		case 10:
			goto tr2
		case 11:
			goto tr3
		case 13:
			goto tr2
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	tr1:
		(m.cs) = 2
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr60:
		(m.cs) = 2
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st2:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof2
		}
	st_case_2:
//line plugins/parsers/influx/machine.go:3254
		switch (m.data)[(m.p)] {
		case 10:
			goto tr8
		case 11:
			goto tr9
		case 13:
			goto tr8
		case 32:
			goto st2
		case 44:
			goto tr8
		case 61:
			goto tr8
		case 92:
			goto tr10
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st2
		}
		goto tr6
	tr6:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st3
	st3:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof3
		}
	st_case_3:
//line plugins/parsers/influx/machine.go:3286
		switch (m.data)[(m.p)] {
		case 32:
			goto tr8
		case 44:
			goto tr8
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	tr2:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr8:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr35:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:24

		err = ErrNameParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr39:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:24

		err = ErrNameParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr43:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:24

		err = ErrNameParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr47:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr105:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:45

		err = ErrTimestampParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr132:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:45

		err = ErrTimestampParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr198:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:45

		err = ErrTimestampParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr423:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:24

		err = ErrNameParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:38

		err = ErrTagParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

//line plugins/parsers/influx/machine.go.rl:31

		err = ErrFieldParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr426:
		(m.cs) = 0
//line plugins/parsers/influx/machine.go.rl:45

		err = ErrTimestampParse
		(m.p)--

		(m.cs) = 258
		{
			(m.p)++
			goto _out
		}

		goto _again
	tr1055:
//line plugins/parsers/influx/machine.go.rl:65

		(m.p)--

		{
			goto st270
		}

		goto st0
//line plugins/parsers/influx/machine.go:3507
	st_case_0:
	st0:
		(m.cs) = 0
		goto _out
	tr12:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st4
	st4:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof4
		}
	st_case_4:
//line plugins/parsers/influx/machine.go:3523
		switch (m.data)[(m.p)] {
		case 34:
			goto st5
		case 45:
			goto tr15
		case 46:
			goto tr16
		case 48:
			goto tr17
		case 70:
			goto tr19
		case 84:
			goto tr20
		case 102:
			goto tr21
		case 116:
			goto tr22
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr18
		}
		goto tr8
	st5:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof5
		}
	st_case_5:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr24
		case 12:
			goto tr8
		case 13:
			goto tr25
		case 34:
			goto tr26
		case 92:
			goto tr27
		}
		goto tr23
	tr23:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st6
	tr24:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st6
	tr29:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st6
	st6:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof6
		}
	st_case_6:
//line plugins/parsers/influx/machine.go:3595
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		goto st6
	tr25:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st7
	st7:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof7
		}
	st_case_7:
//line plugins/parsers/influx/machine.go:3620
		if (m.data)[(m.p)] == 10 {
			goto tr29
		}
		goto tr8
	tr26:
		(m.cs) = 271
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr31:
		(m.cs) = 271
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st271:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof271
		}
	st_case_271:
//line plugins/parsers/influx/machine.go:3660
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto st36
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st272
		}
		goto tr105
	tr535:
		(m.cs) = 272
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr932:
		(m.cs) = 272
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr935:
		(m.cs) = 272
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr939:
		(m.cs) = 272
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st272:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof272
		}
	st_case_272:
//line plugins/parsers/influx/machine.go:3732
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 13:
			goto st33
		case 32:
			goto st272
		case 45:
			goto tr464
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr465
			}
		case (m.data)[(m.p)] >= 9:
			goto st272
		}
		goto tr426
	tr103:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st273
	tr470:
		(m.cs) = 273
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr734:
		(m.cs) = 273
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr952:
		(m.cs) = 273
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr957:
		(m.cs) = 273
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr962:
		(m.cs) = 273
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	st273:
//line plugins/parsers/influx/machine.go.rl:164

		m.finishMetric = true
		(m.cs) = 740
		{
			(m.p)++
			goto _out
		}

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof273
		}
	st_case_273:
//line plugins/parsers/influx/machine.go:3866
		switch (m.data)[(m.p)] {
		case 10:
			goto tr35
		case 11:
			goto tr36
		case 13:
			goto tr35
		case 32:
			goto st8
		case 35:
			goto tr35
		case 44:
			goto tr35
		case 92:
			goto tr37
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st8
		}
		goto tr33
	tr458:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

		goto st8
	st8:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof8
		}
	st_case_8:
//line plugins/parsers/influx/machine.go:3898
		switch (m.data)[(m.p)] {
		case 10:
			goto tr35
		case 11:
			goto tr36
		case 13:
			goto tr35
		case 32:
			goto st8
		case 35:
			goto tr35
		case 44:
			goto tr35
		case 92:
			goto tr37
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st8
		}
		goto tr33
	tr36:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st9
	tr459:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st9
	st9:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof9
		}
	st_case_9:
//line plugins/parsers/influx/machine.go:3940
		switch (m.data)[(m.p)] {
		case 10:
			goto tr39
		case 11:
			goto tr40
		case 13:
			goto tr39
		case 32:
			goto tr38
		case 35:
			goto st1
		case 44:
			goto tr4
		case 92:
			goto tr37
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr38
		}
		goto tr33
	tr38:
		(m.cs) = 10
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st10:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof10
		}
	st_case_10:
//line plugins/parsers/influx/machine.go:3979
		switch (m.data)[(m.p)] {
		case 10:
			goto tr43
		case 11:
			goto tr44
		case 13:
			goto tr43
		case 32:
			goto st10
		case 35:
			goto tr6
		case 44:
			goto tr43
		case 61:
			goto tr33
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st10
		}
		goto tr41
	tr41:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st11
	st11:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof11
		}
	st_case_11:
//line plugins/parsers/influx/machine.go:4013
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr48
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st11
	tr48:
		(m.cs) = 12
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr51:
		(m.cs) = 12
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st12:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof12
		}
	st_case_12:
//line plugins/parsers/influx/machine.go:4069
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr51
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto tr41
	tr4:
		(m.cs) = 13
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr62:
		(m.cs) = 13
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st13:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof13
		}
	st_case_13:
//line plugins/parsers/influx/machine.go:4121
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr2
		case 92:
			goto tr53
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto tr52
	tr52:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st14
	st14:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof14
		}
	st_case_14:
//line plugins/parsers/influx/machine.go:4152
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st14
	tr55:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

		goto st15
	st15:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof15
		}
	st_case_15:
//line plugins/parsers/influx/machine.go:4183
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr2
		case 92:
			goto tr58
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto tr57
	tr57:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st16
	st16:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof16
		}
	st_case_16:
//line plugins/parsers/influx/machine.go:4214
		switch (m.data)[(m.p)] {
		case 10:
			goto tr2
		case 11:
			goto tr61
		case 13:
			goto tr2
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr2
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	tr61:
		(m.cs) = 17
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st17:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof17
		}
	st_case_17:
//line plugins/parsers/influx/machine.go:4253
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr65
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto tr66
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto tr64
	tr64:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st18
	st18:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof18
		}
	st_case_18:
//line plugins/parsers/influx/machine.go:4285
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr68
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st18
	tr68:
		(m.cs) = 19
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr65:
		(m.cs) = 19
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st19:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof19
		}
	st_case_19:
//line plugins/parsers/influx/machine.go:4341
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr65
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto tr66
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto tr64
	tr66:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st20
	st20:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof20
		}
	st_case_20:
//line plugins/parsers/influx/machine.go:4373
		if (m.data)[(m.p)] == 92 {
			goto st21
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st18
	st21:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof21
		}
	st_case_21:
//line plugins/parsers/influx/machine.go:4394
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr68
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st18
	tr58:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st22
	st22:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof22
		}
	st_case_22:
//line plugins/parsers/influx/machine.go:4426
		if (m.data)[(m.p)] == 92 {
			goto st23
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st16
	st23:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof23
		}
	st_case_23:
//line plugins/parsers/influx/machine.go:4447
		switch (m.data)[(m.p)] {
		case 10:
			goto tr2
		case 11:
			goto tr61
		case 13:
			goto tr2
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr2
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	tr53:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st24
	st24:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof24
		}
	st_case_24:
//line plugins/parsers/influx/machine.go:4479
		if (m.data)[(m.p)] == 92 {
			goto st25
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st14
	st25:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof25
		}
	st_case_25:
//line plugins/parsers/influx/machine.go:4500
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st14
	tr49:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st26
	tr425:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st26
	st26:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof26
		}
	st_case_26:
//line plugins/parsers/influx/machine.go:4541
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 34:
			goto st29
		case 44:
			goto tr4
		case 45:
			goto tr74
		case 46:
			goto tr75
		case 48:
			goto tr76
		case 70:
			goto tr78
		case 84:
			goto tr79
		case 92:
			goto st95
		case 102:
			goto tr80
		case 116:
			goto tr81
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr77
			}
		case (m.data)[(m.p)] >= 9:
			goto tr1
		}
		goto st1
	tr3:
		(m.cs) = 27
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st27:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof27
		}
	st_case_27:
//line plugins/parsers/influx/machine.go:4599
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr51
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 61:
			goto st1
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto tr41
	tr45:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st28
	st28:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof28
		}
	st_case_28:
//line plugins/parsers/influx/machine.go:4631
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st11
	st29:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof29
		}
	st_case_29:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr83
		case 10:
			goto tr24
		case 11:
			goto tr84
		case 12:
			goto tr1
		case 13:
			goto tr25
		case 32:
			goto tr83
		case 34:
			goto tr85
		case 44:
			goto tr86
		case 92:
			goto tr87
		}
		goto tr82
	tr82:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st30
	st30:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof30
		}
	st_case_30:
//line plugins/parsers/influx/machine.go:4678
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		}
		goto st30
	tr89:
		(m.cs) = 31
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr83:
		(m.cs) = 31
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr231:
		(m.cs) = 31
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st31:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof31
		}
	st_case_31:
//line plugins/parsers/influx/machine.go:4748
		switch (m.data)[(m.p)] {
		case 9:
			goto st31
		case 10:
			goto tr29
		case 11:
			goto tr96
		case 12:
			goto st2
		case 13:
			goto st7
		case 32:
			goto st31
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr98
		}
		goto tr94
	tr94:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st32
	st32:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof32
		}
	st_case_32:
//line plugins/parsers/influx/machine.go:4783
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		goto st32
	tr97:
		(m.cs) = 274
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr100:
		(m.cs) = 274
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr386:
		(m.cs) = 274
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st274:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof274
		}
	st_case_274:
//line plugins/parsers/influx/machine.go:4857
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st275
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto st36
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st272
		}
		goto st3
	st275:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof275
		}
	st_case_275:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st275
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto tr105
		case 45:
			goto tr467
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr468
			}
		case (m.data)[(m.p)] >= 9:
			goto st272
		}
		goto st3
	tr472:
		(m.cs) = 33
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr736:
		(m.cs) = 33
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr954:
		(m.cs) = 33
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr959:
		(m.cs) = 33
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr964:
		(m.cs) = 33
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st33:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof33
		}
	st_case_33:
//line plugins/parsers/influx/machine.go:4980
		if (m.data)[(m.p)] == 10 {
			goto tr103
		}
		goto st0
	tr467:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st34
	st34:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof34
		}
	st_case_34:
//line plugins/parsers/influx/machine.go:4996
		switch (m.data)[(m.p)] {
		case 32:
			goto tr105
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] < 12:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 10 {
				goto tr105
			}
		case (m.data)[(m.p)] > 13:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st276
			}
		default:
			goto tr105
		}
		goto st3
	tr468:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st276
	st276:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof276
		}
	st_case_276:
//line plugins/parsers/influx/machine.go:5031
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st279
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	tr469:
		(m.cs) = 277
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st277:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof277
		}
	st_case_277:
//line plugins/parsers/influx/machine.go:5075
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 13:
			goto st33
		case 32:
			goto st277
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st277
		}
		goto st0
	tr471:
		(m.cs) = 278
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st278:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof278
		}
	st_case_278:
//line plugins/parsers/influx/machine.go:5106
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st278
		case 13:
			goto st33
		case 32:
			goto st277
		case 44:
			goto tr8
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st277
		}
		goto st3
	tr10:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st35
	st35:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof35
		}
	st_case_35:
//line plugins/parsers/influx/machine.go:5138
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	st279:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof279
		}
	st_case_279:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st280
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st280:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof280
		}
	st_case_280:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st281
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st281:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof281
		}
	st_case_281:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st282
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st282:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof282
		}
	st_case_282:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st283
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st283:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof283
		}
	st_case_283:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st284
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st284:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof284
		}
	st_case_284:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st285
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st285:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof285
		}
	st_case_285:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st286
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st286:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof286
		}
	st_case_286:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st287
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st287:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof287
		}
	st_case_287:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st288
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st288:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof288
		}
	st_case_288:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st289
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st289:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof289
		}
	st_case_289:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st290
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st290:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof290
		}
	st_case_290:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st291
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st291:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof291
		}
	st_case_291:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st292
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st292:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof292
		}
	st_case_292:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st293
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st293:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof293
		}
	st_case_293:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st294
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st294:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof294
		}
	st_case_294:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st295
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st295:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof295
		}
	st_case_295:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st296
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st3
	st296:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof296
		}
	st_case_296:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr471
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr105
		case 61:
			goto tr12
		case 92:
			goto st35
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr469
		}
		goto st3
	tr930:
		(m.cs) = 36
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1046:
		(m.cs) = 36
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1048:
		(m.cs) = 36
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1050:
		(m.cs) = 36
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st36:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof36
		}
	st_case_36:
//line plugins/parsers/influx/machine.go:5740
		switch (m.data)[(m.p)] {
		case 32:
			goto tr8
		case 44:
			goto tr8
		case 61:
			goto tr8
		case 92:
			goto tr10
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto tr6
	tr101:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st37
	st37:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof37
		}
	st_case_37:
//line plugins/parsers/influx/machine.go:5771
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr107
		case 45:
			goto tr108
		case 46:
			goto tr109
		case 48:
			goto tr110
		case 70:
			goto tr112
		case 84:
			goto tr113
		case 92:
			goto st75
		case 102:
			goto tr114
		case 116:
			goto tr115
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr111
		}
		goto st6
	tr107:
		(m.cs) = 297
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st297:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof297
		}
	st_case_297:
//line plugins/parsers/influx/machine.go:5820
		switch (m.data)[(m.p)] {
		case 10:
			goto tr494
		case 12:
			goto st272
		case 13:
			goto tr495
		case 32:
			goto tr493
		case 34:
			goto tr26
		case 44:
			goto tr496
		case 92:
			goto tr27
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr493
		}
		goto tr23
	tr493:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st298
	tr988:
		(m.cs) = 298
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr993:
		(m.cs) = 298
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr996:
		(m.cs) = 298
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr999:
		(m.cs) = 298
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st298:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof298
		}
	st_case_298:
//line plugins/parsers/influx/machine.go:5904
		switch (m.data)[(m.p)] {
		case 10:
			goto tr221
		case 12:
			goto st272
		case 13:
			goto st73
		case 32:
			goto st298
		case 34:
			goto tr31
		case 45:
			goto tr499
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr500
			}
		case (m.data)[(m.p)] >= 9:
			goto st298
		}
		goto st6
	tr494:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st299
	tr221:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st299
	tr639:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr603:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr823:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr829:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr810:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr765:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr798:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr804:
		(m.cs) = 299
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st299:
//line plugins/parsers/influx/machine.go.rl:164

		m.finishMetric = true
		(m.cs) = 740
		{
			(m.p)++
			goto _out
		}

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof299
		}
	st_case_299:
//line plugins/parsers/influx/machine.go:6113
		switch (m.data)[(m.p)] {
		case 9:
			goto st38
		case 10:
			goto tr29
		case 11:
			goto tr117
		case 12:
			goto st8
		case 13:
			goto st7
		case 32:
			goto st38
		case 34:
			goto tr118
		case 35:
			goto st6
		case 44:
			goto st6
		case 92:
			goto tr87
		}
		goto tr82
	st38:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof38
		}
	st_case_38:
		switch (m.data)[(m.p)] {
		case 9:
			goto st38
		case 10:
			goto tr29
		case 11:
			goto tr117
		case 12:
			goto st8
		case 13:
			goto st7
		case 32:
			goto st38
		case 34:
			goto tr118
		case 35:
			goto st6
		case 44:
			goto st6
		case 92:
			goto tr87
		}
		goto tr82
	tr117:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st39
	st39:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof39
		}
	st_case_39:
//line plugins/parsers/influx/machine.go:6176
		switch (m.data)[(m.p)] {
		case 9:
			goto tr119
		case 10:
			goto tr29
		case 11:
			goto tr120
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr119
		case 34:
			goto tr85
		case 35:
			goto st30
		case 44:
			goto tr92
		case 92:
			goto tr87
		}
		goto tr82
	tr119:
		(m.cs) = 40
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st40:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof40
		}
	st_case_40:
//line plugins/parsers/influx/machine.go:6218
		switch (m.data)[(m.p)] {
		case 9:
			goto st40
		case 10:
			goto tr29
		case 11:
			goto tr123
		case 12:
			goto st10
		case 13:
			goto st7
		case 32:
			goto st40
		case 34:
			goto tr124
		case 35:
			goto tr94
		case 44:
			goto st6
		case 61:
			goto tr82
		case 92:
			goto tr125
		}
		goto tr121
	tr121:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st41
	st41:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof41
		}
	st_case_41:
//line plugins/parsers/influx/machine.go:6255
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr127
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		goto st41
	tr127:
		(m.cs) = 42
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr131:
		(m.cs) = 42
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st42:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof42
		}
	st_case_42:
//line plugins/parsers/influx/machine.go:6314
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr131
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto tr125
		}
		goto tr121
	tr124:
		(m.cs) = 300
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr128:
		(m.cs) = 300
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st300:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof300
		}
	st_case_300:
//line plugins/parsers/influx/machine.go:6373
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr502
		case 13:
			goto st33
		case 32:
			goto tr501
		case 44:
			goto tr503
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr501
		}
		goto st11
	tr501:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr566:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr641:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr731:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr743:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr750:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr757:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr825:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr831:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr836:
		(m.cs) = 301
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st301:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof301
		}
	st_case_301:
//line plugins/parsers/influx/machine.go:6609
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr505
		case 13:
			goto st33
		case 32:
			goto st301
		case 44:
			goto tr105
		case 45:
			goto tr467
		case 61:
			goto tr105
		case 92:
			goto tr10
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr468
			}
		case (m.data)[(m.p)] >= 9:
			goto st301
		}
		goto tr6
	tr505:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st302
	st302:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof302
		}
	st_case_302:
//line plugins/parsers/influx/machine.go:6648
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr505
		case 13:
			goto st33
		case 32:
			goto st301
		case 44:
			goto tr105
		case 45:
			goto tr467
		case 61:
			goto tr12
		case 92:
			goto tr10
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr468
			}
		case (m.data)[(m.p)] >= 9:
			goto st301
		}
		goto tr6
	tr502:
		(m.cs) = 303
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr506:
		(m.cs) = 303
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st303:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof303
		}
	st_case_303:
//line plugins/parsers/influx/machine.go:6711
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr506
		case 13:
			goto st33
		case 32:
			goto tr501
		case 44:
			goto tr4
		case 45:
			goto tr507
		case 61:
			goto tr49
		case 92:
			goto tr45
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr508
			}
		case (m.data)[(m.p)] >= 9:
			goto tr501
		}
		goto tr41
	tr507:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st43
	st43:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof43
		}
	st_case_43:
//line plugins/parsers/influx/machine.go:6750
		switch (m.data)[(m.p)] {
		case 10:
			goto tr132
		case 11:
			goto tr48
		case 13:
			goto tr132
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st304
			}
		case (m.data)[(m.p)] >= 9:
			goto tr1
		}
		goto st11
	tr508:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st304
	st304:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof304
		}
	st_case_304:
//line plugins/parsers/influx/machine.go:6787
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st308
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	tr514:
		(m.cs) = 305
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr575:
		(m.cs) = 305
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr509:
		(m.cs) = 305
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr572:
		(m.cs) = 305
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st305:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof305
		}
	st_case_305:
//line plugins/parsers/influx/machine.go:6890
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr513
		case 13:
			goto st33
		case 32:
			goto st305
		case 44:
			goto tr8
		case 61:
			goto tr8
		case 92:
			goto tr10
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st305
		}
		goto tr6
	tr513:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st306
	st306:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof306
		}
	st_case_306:
//line plugins/parsers/influx/machine.go:6922
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr513
		case 13:
			goto st33
		case 32:
			goto st305
		case 44:
			goto tr8
		case 61:
			goto tr12
		case 92:
			goto tr10
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st305
		}
		goto tr6
	tr515:
		(m.cs) = 307
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr510:
		(m.cs) = 307
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st307:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof307
		}
	st_case_307:
//line plugins/parsers/influx/machine.go:6988
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr515
		case 13:
			goto st33
		case 32:
			goto tr514
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr514
		}
		goto tr41
	st308:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof308
		}
	st_case_308:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st309
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st309:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof309
		}
	st_case_309:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st310
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st310:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof310
		}
	st_case_310:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st311
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st311:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof311
		}
	st_case_311:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st312
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st312:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof312
		}
	st_case_312:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st313
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st313:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof313
		}
	st_case_313:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st314
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st314:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof314
		}
	st_case_314:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st315
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st315:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof315
		}
	st_case_315:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st316
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st316:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof316
		}
	st_case_316:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st317
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st317:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof317
		}
	st_case_317:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st318
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st318:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof318
		}
	st_case_318:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st319
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st319:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof319
		}
	st_case_319:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st320
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st320:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof320
		}
	st_case_320:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st321
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st321:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof321
		}
	st_case_321:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st322
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st322:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof322
		}
	st_case_322:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st323
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st323:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof323
		}
	st_case_323:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st324
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st324:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof324
		}
	st_case_324:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st325
			}
		case (m.data)[(m.p)] >= 9:
			goto tr509
		}
		goto st11
	st325:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof325
		}
	st_case_325:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr510
		case 13:
			goto tr472
		case 32:
			goto tr509
		case 44:
			goto tr4
		case 61:
			goto tr49
		case 92:
			goto st28
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr509
		}
		goto st11
	tr503:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr568:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr819:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr737:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr955:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr960:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr965:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1014:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1017:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1020:
		(m.cs) = 44
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st44:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof44
		}
	st_case_44:
//line plugins/parsers/influx/machine.go:7759
		switch (m.data)[(m.p)] {
		case 32:
			goto tr47
		case 44:
			goto tr47
		case 61:
			goto tr47
		case 92:
			goto tr135
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto tr134
	tr134:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st45
	st45:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof45
		}
	st_case_45:
//line plugins/parsers/influx/machine.go:7790
		switch (m.data)[(m.p)] {
		case 32:
			goto tr47
		case 44:
			goto tr47
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st45
	tr137:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st46
	st46:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof46
		}
	st_case_46:
//line plugins/parsers/influx/machine.go:7825
		switch (m.data)[(m.p)] {
		case 32:
			goto tr47
		case 34:
			goto tr139
		case 44:
			goto tr47
		case 45:
			goto tr140
		case 46:
			goto tr141
		case 48:
			goto tr142
		case 61:
			goto tr47
		case 70:
			goto tr144
		case 84:
			goto tr145
		case 92:
			goto tr58
		case 102:
			goto tr146
		case 116:
			goto tr147
		}
		switch {
		case (m.data)[(m.p)] < 12:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 10 {
				goto tr47
			}
		case (m.data)[(m.p)] > 13:
			if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr143
			}
		default:
			goto tr47
		}
		goto tr57
	tr139:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st47
	st47:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof47
		}
	st_case_47:
//line plugins/parsers/influx/machine.go:7876
		switch (m.data)[(m.p)] {
		case 9:
			goto tr149
		case 10:
			goto tr24
		case 11:
			goto tr150
		case 12:
			goto tr60
		case 13:
			goto tr25
		case 32:
			goto tr149
		case 34:
			goto tr151
		case 44:
			goto tr152
		case 61:
			goto tr23
		case 92:
			goto tr153
		}
		goto tr148
	tr148:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st48
	st48:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof48
		}
	st_case_48:
//line plugins/parsers/influx/machine.go:7911
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		}
		goto st48
	tr180:
		(m.cs) = 49
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr155:
		(m.cs) = 49
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr149:
		(m.cs) = 49
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st49:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof49
		}
	st_case_49:
//line plugins/parsers/influx/machine.go:7983
		switch (m.data)[(m.p)] {
		case 9:
			goto st49
		case 10:
			goto tr29
		case 11:
			goto tr162
		case 12:
			goto st2
		case 13:
			goto st7
		case 32:
			goto st49
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr163
		}
		goto tr160
	tr160:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st50
	st50:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof50
		}
	st_case_50:
//line plugins/parsers/influx/machine.go:8018
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		goto st50
	tr165:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st51
	st51:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof51
		}
	st_case_51:
//line plugins/parsers/influx/machine.go:8051
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr107
		case 45:
			goto tr167
		case 46:
			goto tr168
		case 48:
			goto tr169
		case 70:
			goto tr171
		case 84:
			goto tr172
		case 92:
			goto st75
		case 102:
			goto tr173
		case 116:
			goto tr174
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr170
		}
		goto st6
	tr167:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st52
	st52:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof52
		}
	st_case_52:
//line plugins/parsers/influx/machine.go:8093
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 46:
			goto st53
		case 48:
			goto st632
		case 92:
			goto st75
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st633
		}
		goto st6
	tr168:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st53
	st53:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof53
		}
	st_case_53:
//line plugins/parsers/influx/machine.go:8125
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st326
		}
		goto st6
	st326:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof326
		}
	st_case_326:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st326
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	tr925:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st327
	tr533:
		(m.cs) = 327
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr931:
		(m.cs) = 327
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr934:
		(m.cs) = 327
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr938:
		(m.cs) = 327
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st327:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof327
		}
	st_case_327:
//line plugins/parsers/influx/machine.go:8239
		switch (m.data)[(m.p)] {
		case 10:
			goto tr275
		case 12:
			goto st272
		case 13:
			goto st103
		case 32:
			goto st327
		case 34:
			goto tr31
		case 45:
			goto tr541
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr542
			}
		case (m.data)[(m.p)] >= 9:
			goto st327
		}
		goto st6
	tr669:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st328
	tr275:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st328
	tr534:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr678:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr741:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr748:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr755:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr900:
		(m.cs) = 328
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	st328:
//line plugins/parsers/influx/machine.go.rl:164

		m.finishMetric = true
		(m.cs) = 740
		{
			(m.p)++
			goto _out
		}

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof328
		}
	st_case_328:
//line plugins/parsers/influx/machine.go:8410
		switch (m.data)[(m.p)] {
		case 9:
			goto st165
		case 10:
			goto tr29
		case 11:
			goto tr339
		case 12:
			goto st8
		case 13:
			goto st7
		case 32:
			goto st165
		case 34:
			goto tr118
		case 35:
			goto st6
		case 44:
			goto st6
		case 92:
			goto tr340
		}
		goto tr337
	tr337:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st54
	st54:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof54
		}
	st_case_54:
//line plugins/parsers/influx/machine.go:8445
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		}
		goto st54
	tr181:
		(m.cs) = 55
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st55:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof55
		}
	st_case_55:
//line plugins/parsers/influx/machine.go:8485
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr185
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 61:
			goto st54
		case 92:
			goto tr186
		}
		goto tr184
	tr184:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st56
	st56:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof56
		}
	st_case_56:
//line plugins/parsers/influx/machine.go:8520
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr188
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		goto st56
	tr188:
		(m.cs) = 57
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr185:
		(m.cs) = 57
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st57:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof57
		}
	st_case_57:
//line plugins/parsers/influx/machine.go:8579
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr185
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto tr186
		}
		goto tr184
	tr182:
		(m.cs) = 58
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr158:
		(m.cs) = 58
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr152:
		(m.cs) = 58
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st58:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof58
		}
	st_case_58:
//line plugins/parsers/influx/machine.go:8651
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr192
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr193
		}
		goto tr191
	tr191:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st59
	st59:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof59
		}
	st_case_59:
//line plugins/parsers/influx/machine.go:8684
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr195
		case 44:
			goto st6
		case 61:
			goto tr196
		case 92:
			goto st70
		}
		goto st59
	tr192:
		(m.cs) = 329
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr195:
		(m.cs) = 329
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st329:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof329
		}
	st_case_329:
//line plugins/parsers/influx/machine.go:8741
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st330
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto st36
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st272
		}
		goto st14
	st330:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof330
		}
	st_case_330:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st330
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto tr198
		case 45:
			goto tr544
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr545
			}
		case (m.data)[(m.p)] >= 9:
			goto st272
		}
		goto st14
	tr544:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st60
	st60:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof60
		}
	st_case_60:
//line plugins/parsers/influx/machine.go:8805
		switch (m.data)[(m.p)] {
		case 32:
			goto tr198
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] < 12:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 10 {
				goto tr198
			}
		case (m.data)[(m.p)] > 13:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st331
			}
		default:
			goto tr198
		}
		goto st14
	tr545:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st331
	st331:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof331
		}
	st_case_331:
//line plugins/parsers/influx/machine.go:8840
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st333
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	tr546:
		(m.cs) = 332
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st332:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof332
		}
	st_case_332:
//line plugins/parsers/influx/machine.go:8884
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st332
		case 13:
			goto st33
		case 32:
			goto st277
		case 44:
			goto tr2
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st277
		}
		goto st14
	st333:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof333
		}
	st_case_333:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st334
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st334:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof334
		}
	st_case_334:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st335
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st335:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof335
		}
	st_case_335:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st336
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st336:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof336
		}
	st_case_336:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st337
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st337:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof337
		}
	st_case_337:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st338
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st338:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof338
		}
	st_case_338:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st339
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st339:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof339
		}
	st_case_339:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st340
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st340:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof340
		}
	st_case_340:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st341
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st341:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof341
		}
	st_case_341:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st342
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st342:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof342
		}
	st_case_342:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st343
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st343:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof343
		}
	st_case_343:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st344
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st344:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof344
		}
	st_case_344:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st345
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st345:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof345
		}
	st_case_345:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st346
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st346:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof346
		}
	st_case_346:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st347
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st347:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof347
		}
	st_case_347:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st348
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st348:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof348
		}
	st_case_348:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st349
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st349:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof349
		}
	st_case_349:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st350
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st14
	st350:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof350
		}
	st_case_350:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr546
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr198
		case 61:
			goto tr55
		case 92:
			goto st24
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr469
		}
		goto st14
	tr196:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

		goto st61
	st61:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof61
		}
	st_case_61:
//line plugins/parsers/influx/machine.go:9451
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr151
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr153
		}
		goto tr148
	tr151:
		(m.cs) = 351
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr157:
		(m.cs) = 351
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st351:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof351
		}
	st_case_351:
//line plugins/parsers/influx/machine.go:9508
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr567
		case 13:
			goto st33
		case 32:
			goto tr566
		case 44:
			goto tr568
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr566
		}
		goto st16
	tr567:
		(m.cs) = 352
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr735:
		(m.cs) = 352
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr953:
		(m.cs) = 352
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr958:
		(m.cs) = 352
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr963:
		(m.cs) = 352
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st352:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof352
		}
	st_case_352:
//line plugins/parsers/influx/machine.go:9639
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr569
		case 13:
			goto st33
		case 32:
			goto tr566
		case 44:
			goto tr62
		case 45:
			goto tr570
		case 61:
			goto tr132
		case 92:
			goto tr66
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr571
			}
		case (m.data)[(m.p)] >= 9:
			goto tr566
		}
		goto tr64
	tr594:
		(m.cs) = 353
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr569:
		(m.cs) = 353
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st353:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof353
		}
	st_case_353:
//line plugins/parsers/influx/machine.go:9702
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr569
		case 13:
			goto st33
		case 32:
			goto tr566
		case 44:
			goto tr62
		case 45:
			goto tr570
		case 61:
			goto tr12
		case 92:
			goto tr66
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr571
			}
		case (m.data)[(m.p)] >= 9:
			goto tr566
		}
		goto tr64
	tr570:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st62
	st62:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof62
		}
	st_case_62:
//line plugins/parsers/influx/machine.go:9741
		switch (m.data)[(m.p)] {
		case 10:
			goto tr132
		case 11:
			goto tr68
		case 13:
			goto tr132
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st354
			}
		case (m.data)[(m.p)] >= 9:
			goto tr60
		}
		goto st18
	tr571:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st354
	st354:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof354
		}
	st_case_354:
//line plugins/parsers/influx/machine.go:9778
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st356
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	tr576:
		(m.cs) = 355
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr573:
		(m.cs) = 355
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st355:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof355
		}
	st_case_355:
//line plugins/parsers/influx/machine.go:9849
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr576
		case 13:
			goto st33
		case 32:
			goto tr575
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto tr66
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr575
		}
		goto tr64
	st356:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof356
		}
	st_case_356:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st357
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st357:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof357
		}
	st_case_357:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st358
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st358:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof358
		}
	st_case_358:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st359
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st359:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof359
		}
	st_case_359:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st360
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st360:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof360
		}
	st_case_360:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st361
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st361:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof361
		}
	st_case_361:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st362
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st362:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof362
		}
	st_case_362:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st363
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st363:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof363
		}
	st_case_363:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st364
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st364:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof364
		}
	st_case_364:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st365
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st365:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof365
		}
	st_case_365:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st366
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st366:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof366
		}
	st_case_366:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st367
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st367:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof367
		}
	st_case_367:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st368
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st368:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof368
		}
	st_case_368:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st369
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st369:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof369
		}
	st_case_369:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st370
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st370:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof370
		}
	st_case_370:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st371
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st371:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof371
		}
	st_case_371:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st372
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st372:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof372
		}
	st_case_372:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st373
			}
		case (m.data)[(m.p)] >= 9:
			goto tr572
		}
		goto st18
	st373:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof373
		}
	st_case_373:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr573
		case 13:
			goto tr472
		case 32:
			goto tr572
		case 44:
			goto tr62
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr572
		}
		goto st18
	tr153:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st63
	st63:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof63
		}
	st_case_63:
//line plugins/parsers/influx/machine.go:10416
		switch (m.data)[(m.p)] {
		case 34:
			goto st48
		case 92:
			goto st64
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st16
	st64:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof64
		}
	st_case_64:
//line plugins/parsers/influx/machine.go:10440
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		}
		goto st48
	tr156:
		(m.cs) = 65
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr150:
		(m.cs) = 65
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st65:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof65
		}
	st_case_65:
//line plugins/parsers/influx/machine.go:10499
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr203
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr204
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto tr205
		}
		goto tr202
	tr202:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st66
	st66:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof66
		}
	st_case_66:
//line plugins/parsers/influx/machine.go:10534
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr207
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		goto st66
	tr207:
		(m.cs) = 67
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr203:
		(m.cs) = 67
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st67:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof67
		}
	st_case_67:
//line plugins/parsers/influx/machine.go:10593
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr203
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr204
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto tr205
		}
		goto tr202
	tr204:
		(m.cs) = 374
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr208:
		(m.cs) = 374
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st374:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof374
		}
	st_case_374:
//line plugins/parsers/influx/machine.go:10652
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr594
		case 13:
			goto st33
		case 32:
			goto tr566
		case 44:
			goto tr568
		case 61:
			goto tr12
		case 92:
			goto st20
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr566
		}
		goto st18
	tr205:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st68
	st68:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof68
		}
	st_case_68:
//line plugins/parsers/influx/machine.go:10684
		switch (m.data)[(m.p)] {
		case 34:
			goto st66
		case 92:
			goto st69
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st18
	st69:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof69
		}
	st_case_69:
//line plugins/parsers/influx/machine.go:10708
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr207
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		goto st66
	tr193:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st70
	st70:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof70
		}
	st_case_70:
//line plugins/parsers/influx/machine.go:10743
		switch (m.data)[(m.p)] {
		case 34:
			goto st59
		case 92:
			goto st71
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st14
	st71:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof71
		}
	st_case_71:
//line plugins/parsers/influx/machine.go:10767
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr195
		case 44:
			goto st6
		case 61:
			goto tr196
		case 92:
			goto st70
		}
		goto st59
	tr189:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st72
	tr346:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st72
	st72:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof72
		}
	st_case_72:
//line plugins/parsers/influx/machine.go:10810
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr212
		case 44:
			goto tr182
		case 45:
			goto tr213
		case 46:
			goto tr214
		case 48:
			goto tr215
		case 70:
			goto tr217
		case 84:
			goto tr218
		case 92:
			goto st156
		case 102:
			goto tr219
		case 116:
			goto tr220
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr216
		}
		goto st54
	tr212:
		(m.cs) = 375
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st375:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof375
		}
	st_case_375:
//line plugins/parsers/influx/machine.go:10867
		switch (m.data)[(m.p)] {
		case 9:
			goto tr595
		case 10:
			goto tr494
		case 11:
			goto tr596
		case 12:
			goto tr501
		case 13:
			goto tr495
		case 32:
			goto tr595
		case 34:
			goto tr85
		case 44:
			goto tr597
		case 92:
			goto tr87
		}
		goto tr82
	tr626:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr595:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr769:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr638:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr764:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr797:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr803:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr809:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr822:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr828:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr834:
		(m.cs) = 376
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st376:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof376
		}
	st_case_376:
//line plugins/parsers/influx/machine.go:11121
		switch (m.data)[(m.p)] {
		case 9:
			goto st376
		case 10:
			goto tr221
		case 11:
			goto tr599
		case 12:
			goto st301
		case 13:
			goto st73
		case 32:
			goto st376
		case 34:
			goto tr97
		case 44:
			goto st6
		case 45:
			goto tr600
		case 61:
			goto st6
		case 92:
			goto tr98
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr601
		}
		goto tr94
	tr599:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st377
	st377:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof377
		}
	st_case_377:
//line plugins/parsers/influx/machine.go:11161
		switch (m.data)[(m.p)] {
		case 9:
			goto st376
		case 10:
			goto tr221
		case 11:
			goto tr599
		case 12:
			goto st301
		case 13:
			goto st73
		case 32:
			goto st376
		case 34:
			goto tr97
		case 44:
			goto st6
		case 45:
			goto tr600
		case 61:
			goto tr101
		case 92:
			goto tr98
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr601
		}
		goto tr94
	tr495:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st73
	tr605:
		(m.cs) = 73
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr642:
		(m.cs) = 73
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr800:
		(m.cs) = 73
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr806:
		(m.cs) = 73
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr812:
		(m.cs) = 73
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st73:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof73
		}
	st_case_73:
//line plugins/parsers/influx/machine.go:11266
		if (m.data)[(m.p)] == 10 {
			goto tr221
		}
		goto tr8
	tr600:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st74
	st74:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof74
		}
	st_case_74:
//line plugins/parsers/influx/machine.go:11282
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr105
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st378
		}
		goto st32
	tr601:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st378
	st378:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof378
		}
	st_case_378:
//line plugins/parsers/influx/machine.go:11318
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st381
		}
		goto st32
	tr602:
		(m.cs) = 379
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st379:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof379
		}
	st_case_379:
//line plugins/parsers/influx/machine.go:11363
		switch (m.data)[(m.p)] {
		case 10:
			goto tr221
		case 12:
			goto st277
		case 13:
			goto st73
		case 32:
			goto st379
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto st379
		}
		goto st6
	tr27:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st75
	st75:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof75
		}
	st_case_75:
//line plugins/parsers/influx/machine.go:11393
		switch (m.data)[(m.p)] {
		case 34:
			goto st6
		case 92:
			goto st6
		}
		goto tr8
	tr604:
		(m.cs) = 380
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st380:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof380
		}
	st_case_380:
//line plugins/parsers/influx/machine.go:11419
		switch (m.data)[(m.p)] {
		case 9:
			goto st379
		case 10:
			goto tr221
		case 11:
			goto st380
		case 12:
			goto st277
		case 13:
			goto st73
		case 32:
			goto st379
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		goto st32
	tr98:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st76
	st76:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof76
		}
	st_case_76:
//line plugins/parsers/influx/machine.go:11454
		switch (m.data)[(m.p)] {
		case 34:
			goto st32
		case 92:
			goto st32
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	st381:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof381
		}
	st_case_381:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st382
		}
		goto st32
	st382:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof382
		}
	st_case_382:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st383
		}
		goto st32
	st383:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof383
		}
	st_case_383:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st384
		}
		goto st32
	st384:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof384
		}
	st_case_384:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st385
		}
		goto st32
	st385:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof385
		}
	st_case_385:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st386
		}
		goto st32
	st386:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof386
		}
	st_case_386:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st387
		}
		goto st32
	st387:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof387
		}
	st_case_387:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st388
		}
		goto st32
	st388:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof388
		}
	st_case_388:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st389
		}
		goto st32
	st389:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof389
		}
	st_case_389:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st390
		}
		goto st32
	st390:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof390
		}
	st_case_390:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st391
		}
		goto st32
	st391:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof391
		}
	st_case_391:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st392
		}
		goto st32
	st392:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof392
		}
	st_case_392:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st393
		}
		goto st32
	st393:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof393
		}
	st_case_393:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st394
		}
		goto st32
	st394:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof394
		}
	st_case_394:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st395
		}
		goto st32
	st395:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof395
		}
	st_case_395:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st396
		}
		goto st32
	st396:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof396
		}
	st_case_396:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st397
		}
		goto st32
	st397:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof397
		}
	st_case_397:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st398
		}
		goto st32
	st398:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof398
		}
	st_case_398:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr602
		case 10:
			goto tr603
		case 11:
			goto tr604
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto st76
		}
		goto st32
	tr596:
		(m.cs) = 399
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr640:
		(m.cs) = 399
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr824:
		(m.cs) = 399
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr830:
		(m.cs) = 399
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr835:
		(m.cs) = 399
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st399:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof399
		}
	st_case_399:
//line plugins/parsers/influx/machine.go:12139
		switch (m.data)[(m.p)] {
		case 9:
			goto tr626
		case 10:
			goto tr221
		case 11:
			goto tr627
		case 12:
			goto tr501
		case 13:
			goto st73
		case 32:
			goto tr626
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 45:
			goto tr628
		case 61:
			goto st30
		case 92:
			goto tr125
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr629
		}
		goto tr121
	tr627:
		(m.cs) = 400
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st400:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof400
		}
	st_case_400:
//line plugins/parsers/influx/machine.go:12190
		switch (m.data)[(m.p)] {
		case 9:
			goto tr626
		case 10:
			goto tr221
		case 11:
			goto tr627
		case 12:
			goto tr501
		case 13:
			goto st73
		case 32:
			goto tr626
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 45:
			goto tr628
		case 61:
			goto tr129
		case 92:
			goto tr125
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr629
		}
		goto tr121
	tr92:
		(m.cs) = 77
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr86:
		(m.cs) = 77
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr233:
		(m.cs) = 77
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st77:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof77
		}
	st_case_77:
//line plugins/parsers/influx/machine.go:12267
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr192
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr224
		}
		goto tr223
	tr223:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st78
	st78:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof78
		}
	st_case_78:
//line plugins/parsers/influx/machine.go:12300
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr195
		case 44:
			goto st6
		case 61:
			goto tr226
		case 92:
			goto st88
		}
		goto st78
	tr226:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

		goto st79
	st79:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof79
		}
	st_case_79:
//line plugins/parsers/influx/machine.go:12333
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr151
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr229
		}
		goto tr228
	tr228:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st80
	st80:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof80
		}
	st_case_80:
//line plugins/parsers/influx/machine.go:12366
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		}
		goto st80
	tr232:
		(m.cs) = 81
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st81:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof81
		}
	st_case_81:
//line plugins/parsers/influx/machine.go:12408
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr236
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr204
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto tr237
		}
		goto tr235
	tr235:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st82
	st82:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof82
		}
	st_case_82:
//line plugins/parsers/influx/machine.go:12443
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr239
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		goto st82
	tr239:
		(m.cs) = 83
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr236:
		(m.cs) = 83
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st83:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof83
		}
	st_case_83:
//line plugins/parsers/influx/machine.go:12502
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr236
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr204
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto tr237
		}
		goto tr235
	tr237:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st84
	st84:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof84
		}
	st_case_84:
//line plugins/parsers/influx/machine.go:12537
		switch (m.data)[(m.p)] {
		case 34:
			goto st82
		case 92:
			goto st85
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st18
	st85:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof85
		}
	st_case_85:
//line plugins/parsers/influx/machine.go:12561
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr239
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		goto st82
	tr229:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st86
	st86:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof86
		}
	st_case_86:
//line plugins/parsers/influx/machine.go:12596
		switch (m.data)[(m.p)] {
		case 34:
			goto st80
		case 92:
			goto st87
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st16
	st87:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof87
		}
	st_case_87:
//line plugins/parsers/influx/machine.go:12620
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		}
		goto st80
	tr224:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st88
	st88:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof88
		}
	st_case_88:
//line plugins/parsers/influx/machine.go:12655
		switch (m.data)[(m.p)] {
		case 34:
			goto st78
		case 92:
			goto st89
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st14
	st89:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof89
		}
	st_case_89:
//line plugins/parsers/influx/machine.go:12679
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr195
		case 44:
			goto st6
		case 61:
			goto tr226
		case 92:
			goto st88
		}
		goto st78
	tr628:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st90
	st90:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof90
		}
	st_case_90:
//line plugins/parsers/influx/machine.go:12712
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr127
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st401
		}
		goto st41
	tr629:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st401
	st401:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof401
		}
	st_case_401:
//line plugins/parsers/influx/machine.go:12750
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st545
		}
		goto st41
	tr635:
		(m.cs) = 402
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr776:
		(m.cs) = 402
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr630:
		(m.cs) = 402
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr773:
		(m.cs) = 402
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st402:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof402
		}
	st_case_402:
//line plugins/parsers/influx/machine.go:12854
		switch (m.data)[(m.p)] {
		case 9:
			goto st402
		case 10:
			goto tr221
		case 11:
			goto tr634
		case 12:
			goto st305
		case 13:
			goto st73
		case 32:
			goto st402
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr98
		}
		goto tr94
	tr634:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st403
	st403:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof403
		}
	st_case_403:
//line plugins/parsers/influx/machine.go:12889
		switch (m.data)[(m.p)] {
		case 9:
			goto st402
		case 10:
			goto tr221
		case 11:
			goto tr634
		case 12:
			goto st305
		case 13:
			goto st73
		case 32:
			goto st402
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto tr98
		}
		goto tr94
	tr636:
		(m.cs) = 404
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr631:
		(m.cs) = 404
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st404:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof404
		}
	st_case_404:
//line plugins/parsers/influx/machine.go:12958
		switch (m.data)[(m.p)] {
		case 9:
			goto tr635
		case 10:
			goto tr221
		case 11:
			goto tr636
		case 12:
			goto tr514
		case 13:
			goto st73
		case 32:
			goto tr635
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto tr125
		}
		goto tr121
	tr129:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st91
	tr383:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st91
	st91:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof91
		}
	st_case_91:
//line plugins/parsers/influx/machine.go:13003
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr212
		case 44:
			goto tr92
		case 45:
			goto tr245
		case 46:
			goto tr246
		case 48:
			goto tr247
		case 70:
			goto tr249
		case 84:
			goto tr250
		case 92:
			goto st141
		case 102:
			goto tr251
		case 116:
			goto tr252
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr248
		}
		goto st30
	tr90:
		(m.cs) = 92
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr84:
		(m.cs) = 92
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st92:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof92
		}
	st_case_92:
//line plugins/parsers/influx/machine.go:13077
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr131
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 61:
			goto st30
		case 92:
			goto tr125
		}
		goto tr121
	tr125:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st93
	st93:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof93
		}
	st_case_93:
//line plugins/parsers/influx/machine.go:13112
		switch (m.data)[(m.p)] {
		case 34:
			goto st41
		case 92:
			goto st41
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st11
	tr245:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st94
	st94:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof94
		}
	st_case_94:
//line plugins/parsers/influx/machine.go:13139
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 46:
			goto st96
		case 48:
			goto st533
		case 92:
			goto st141
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st536
		}
		goto st30
	tr85:
		(m.cs) = 405
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr91:
		(m.cs) = 405
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr118:
		(m.cs) = 405
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st405:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof405
		}
	st_case_405:
//line plugins/parsers/influx/machine.go:13220
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr637
		case 13:
			goto st33
		case 32:
			goto tr501
		case 44:
			goto tr503
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr501
		}
		goto st1
	tr637:
		(m.cs) = 406
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr818:
		(m.cs) = 406
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1013:
		(m.cs) = 406
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1016:
		(m.cs) = 406
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1019:
		(m.cs) = 406
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st406:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof406
		}
	st_case_406:
//line plugins/parsers/influx/machine.go:13349
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr506
		case 13:
			goto st33
		case 32:
			goto tr501
		case 44:
			goto tr4
		case 45:
			goto tr507
		case 61:
			goto st1
		case 92:
			goto tr45
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr508
			}
		case (m.data)[(m.p)] >= 9:
			goto tr501
		}
		goto tr41
	tr37:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st95
	tr460:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st95
	st95:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof95
		}
	st_case_95:
//line plugins/parsers/influx/machine.go:13398
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto st0
			}
		case (m.data)[(m.p)] >= 9:
			goto st0
		}
		goto st1
	tr246:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st96
	st96:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof96
		}
	st_case_96:
//line plugins/parsers/influx/machine.go:13419
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st407
		}
		goto st30
	st407:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof407
		}
	st_case_407:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st407
		}
		goto st30
	tr597:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr643:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr767:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr801:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr807:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr813:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr826:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr832:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr837:
		(m.cs) = 97
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st97:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof97
		}
	st_case_97:
//line plugins/parsers/influx/machine.go:13683
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr258
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr259
		}
		goto tr257
	tr257:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st98
	st98:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof98
		}
	st_case_98:
//line plugins/parsers/influx/machine.go:13716
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr261
		case 44:
			goto st6
		case 61:
			goto tr262
		case 92:
			goto st137
		}
		goto st98
	tr258:
		(m.cs) = 408
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr261:
		(m.cs) = 408
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st408:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof408
		}
	st_case_408:
//line plugins/parsers/influx/machine.go:13773
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st409
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto st36
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st272
		}
		goto st45
	st409:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof409
		}
	st_case_409:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st409
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto tr132
		case 45:
			goto tr646
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto tr647
			}
		case (m.data)[(m.p)] >= 9:
			goto st272
		}
		goto st45
	tr646:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st99
	st99:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof99
		}
	st_case_99:
//line plugins/parsers/influx/machine.go:13837
		switch (m.data)[(m.p)] {
		case 32:
			goto tr132
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] < 12:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 10 {
				goto tr132
			}
		case (m.data)[(m.p)] > 13:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st410
			}
		default:
			goto tr132
		}
		goto st45
	tr647:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st410
	st410:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof410
		}
	st_case_410:
//line plugins/parsers/influx/machine.go:13872
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st412
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	tr648:
		(m.cs) = 411
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st411:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof411
		}
	st_case_411:
//line plugins/parsers/influx/machine.go:13916
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto st411
		case 13:
			goto st33
		case 32:
			goto st277
		case 44:
			goto tr47
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st277
		}
		goto st45
	tr135:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st100
	st100:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof100
		}
	st_case_100:
//line plugins/parsers/influx/machine.go:13948
		if (m.data)[(m.p)] == 92 {
			goto st101
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st45
	st101:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof101
		}
	st_case_101:
//line plugins/parsers/influx/machine.go:13969
		switch (m.data)[(m.p)] {
		case 32:
			goto tr47
		case 44:
			goto tr47
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st45
	st412:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof412
		}
	st_case_412:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st413
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st413:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof413
		}
	st_case_413:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st414
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st414:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof414
		}
	st_case_414:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st415
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st415:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof415
		}
	st_case_415:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st416
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st416:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof416
		}
	st_case_416:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st417
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st417:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof417
		}
	st_case_417:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st418
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st418:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof418
		}
	st_case_418:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st419
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st419:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof419
		}
	st_case_419:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st420
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st420:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof420
		}
	st_case_420:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st421
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st421:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof421
		}
	st_case_421:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st422
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st422:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof422
		}
	st_case_422:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st423
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st423:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof423
		}
	st_case_423:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st424
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st424:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof424
		}
	st_case_424:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st425
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st425:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof425
		}
	st_case_425:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st426
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st426:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof426
		}
	st_case_426:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st427
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st427:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof427
		}
	st_case_427:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st428
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st428:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof428
		}
	st_case_428:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st429
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto st45
	st429:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof429
		}
	st_case_429:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 11:
			goto tr648
		case 13:
			goto tr472
		case 32:
			goto tr469
		case 44:
			goto tr132
		case 61:
			goto tr137
		case 92:
			goto st100
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr469
		}
		goto st45
	tr262:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st102
	st102:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof102
		}
	st_case_102:
//line plugins/parsers/influx/machine.go:14539
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr266
		case 44:
			goto st6
		case 45:
			goto tr267
		case 46:
			goto tr268
		case 48:
			goto tr269
		case 61:
			goto st6
		case 70:
			goto tr271
		case 84:
			goto tr272
		case 92:
			goto tr229
		case 102:
			goto tr273
		case 116:
			goto tr274
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr270
		}
		goto tr228
	tr266:
		(m.cs) = 430
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st430:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof430
		}
	st_case_430:
//line plugins/parsers/influx/machine.go:14600
		switch (m.data)[(m.p)] {
		case 9:
			goto tr668
		case 10:
			goto tr669
		case 11:
			goto tr670
		case 12:
			goto tr566
		case 13:
			goto tr671
		case 32:
			goto tr668
		case 34:
			goto tr151
		case 44:
			goto tr672
		case 61:
			goto tr23
		case 92:
			goto tr153
		}
		goto tr148
	tr863:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr701:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr668:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr859:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr729:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr740:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr747:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr754:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr891:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr895:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr899:
		(m.cs) = 431
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st431:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof431
		}
	st_case_431:
//line plugins/parsers/influx/machine.go:14856
		switch (m.data)[(m.p)] {
		case 9:
			goto st431
		case 10:
			goto tr275
		case 11:
			goto tr674
		case 12:
			goto st301
		case 13:
			goto st103
		case 32:
			goto st431
		case 34:
			goto tr97
		case 44:
			goto st6
		case 45:
			goto tr675
		case 61:
			goto st6
		case 92:
			goto tr163
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr676
		}
		goto tr160
	tr674:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st432
	st432:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof432
		}
	st_case_432:
//line plugins/parsers/influx/machine.go:14896
		switch (m.data)[(m.p)] {
		case 9:
			goto st431
		case 10:
			goto tr275
		case 11:
			goto tr674
		case 12:
			goto st301
		case 13:
			goto st103
		case 32:
			goto st431
		case 34:
			goto tr97
		case 44:
			goto st6
		case 45:
			goto tr675
		case 61:
			goto tr165
		case 92:
			goto tr163
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr676
		}
		goto tr160
	tr671:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st103
	tr680:
		(m.cs) = 103
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr536:
		(m.cs) = 103
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr744:
		(m.cs) = 103
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr751:
		(m.cs) = 103
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr758:
		(m.cs) = 103
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st103:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof103
		}
	st_case_103:
//line plugins/parsers/influx/machine.go:15001
		if (m.data)[(m.p)] == 10 {
			goto tr275
		}
		goto tr8
	tr675:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st104
	st104:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof104
		}
	st_case_104:
//line plugins/parsers/influx/machine.go:15017
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr105
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st433
		}
		goto st50
	tr676:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st433
	st433:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof433
		}
	st_case_433:
//line plugins/parsers/influx/machine.go:15053
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st436
		}
		goto st50
	tr677:
		(m.cs) = 434
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st434:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof434
		}
	st_case_434:
//line plugins/parsers/influx/machine.go:15098
		switch (m.data)[(m.p)] {
		case 10:
			goto tr275
		case 12:
			goto st277
		case 13:
			goto st103
		case 32:
			goto st434
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto st434
		}
		goto st6
	tr679:
		(m.cs) = 435
//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st435:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof435
		}
	st_case_435:
//line plugins/parsers/influx/machine.go:15135
		switch (m.data)[(m.p)] {
		case 9:
			goto st434
		case 10:
			goto tr275
		case 11:
			goto st435
		case 12:
			goto st277
		case 13:
			goto st103
		case 32:
			goto st434
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		goto st50
	tr163:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st105
	st105:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof105
		}
	st_case_105:
//line plugins/parsers/influx/machine.go:15170
		switch (m.data)[(m.p)] {
		case 34:
			goto st50
		case 92:
			goto st50
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	st436:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof436
		}
	st_case_436:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st437
		}
		goto st50
	st437:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof437
		}
	st_case_437:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st438
		}
		goto st50
	st438:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof438
		}
	st_case_438:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st439
		}
		goto st50
	st439:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof439
		}
	st_case_439:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st440
		}
		goto st50
	st440:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof440
		}
	st_case_440:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st441
		}
		goto st50
	st441:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof441
		}
	st_case_441:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st442
		}
		goto st50
	st442:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof442
		}
	st_case_442:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st443
		}
		goto st50
	st443:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof443
		}
	st_case_443:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st444
		}
		goto st50
	st444:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof444
		}
	st_case_444:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st445
		}
		goto st50
	st445:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof445
		}
	st_case_445:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st446
		}
		goto st50
	st446:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof446
		}
	st_case_446:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st447
		}
		goto st50
	st447:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof447
		}
	st_case_447:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st448
		}
		goto st50
	st448:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof448
		}
	st_case_448:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st449
		}
		goto st50
	st449:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof449
		}
	st_case_449:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st450
		}
		goto st50
	st450:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof450
		}
	st_case_450:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st451
		}
		goto st50
	st451:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof451
		}
	st_case_451:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st452
		}
		goto st50
	st452:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof452
		}
	st_case_452:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st453
		}
		goto st50
	st453:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof453
		}
	st_case_453:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr677
		case 10:
			goto tr678
		case 11:
			goto tr679
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto st105
		}
		goto st50
	tr670:
		(m.cs) = 454
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr730:
		(m.cs) = 454
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr742:
		(m.cs) = 454
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr749:
		(m.cs) = 454
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr756:
		(m.cs) = 454
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st454:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof454
		}
	st_case_454:
//line plugins/parsers/influx/machine.go:15855
		switch (m.data)[(m.p)] {
		case 9:
			goto tr701
		case 10:
			goto tr275
		case 11:
			goto tr702
		case 12:
			goto tr566
		case 13:
			goto st103
		case 32:
			goto tr701
		case 34:
			goto tr204
		case 44:
			goto tr158
		case 45:
			goto tr703
		case 61:
			goto st6
		case 92:
			goto tr205
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr704
		}
		goto tr202
	tr702:
		(m.cs) = 455
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st455:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof455
		}
	st_case_455:
//line plugins/parsers/influx/machine.go:15906
		switch (m.data)[(m.p)] {
		case 9:
			goto tr701
		case 10:
			goto tr275
		case 11:
			goto tr702
		case 12:
			goto tr566
		case 13:
			goto st103
		case 32:
			goto tr701
		case 34:
			goto tr204
		case 44:
			goto tr158
		case 45:
			goto tr703
		case 61:
			goto tr165
		case 92:
			goto tr205
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr704
		}
		goto tr202
	tr703:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st106
	st106:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof106
		}
	st_case_106:
//line plugins/parsers/influx/machine.go:15946
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr207
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st456
		}
		goto st66
	tr704:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st456
	st456:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof456
		}
	st_case_456:
//line plugins/parsers/influx/machine.go:15984
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st460
		}
		goto st66
	tr870:
		(m.cs) = 457
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr710:
		(m.cs) = 457
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr867:
		(m.cs) = 457
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr705:
		(m.cs) = 457
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st457:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof457
		}
	st_case_457:
//line plugins/parsers/influx/machine.go:16088
		switch (m.data)[(m.p)] {
		case 9:
			goto st457
		case 10:
			goto tr275
		case 11:
			goto tr709
		case 12:
			goto st305
		case 13:
			goto st103
		case 32:
			goto st457
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr163
		}
		goto tr160
	tr709:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st458
	st458:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof458
		}
	st_case_458:
//line plugins/parsers/influx/machine.go:16123
		switch (m.data)[(m.p)] {
		case 9:
			goto st457
		case 10:
			goto tr275
		case 11:
			goto tr709
		case 12:
			goto st305
		case 13:
			goto st103
		case 32:
			goto st457
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto tr163
		}
		goto tr160
	tr711:
		(m.cs) = 459
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr706:
		(m.cs) = 459
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st459:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof459
		}
	st_case_459:
//line plugins/parsers/influx/machine.go:16192
		switch (m.data)[(m.p)] {
		case 9:
			goto tr710
		case 10:
			goto tr275
		case 11:
			goto tr711
		case 12:
			goto tr575
		case 13:
			goto st103
		case 32:
			goto tr710
		case 34:
			goto tr204
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto tr205
		}
		goto tr202
	st460:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof460
		}
	st_case_460:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st461
		}
		goto st66
	st461:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof461
		}
	st_case_461:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st462
		}
		goto st66
	st462:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof462
		}
	st_case_462:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st463
		}
		goto st66
	st463:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof463
		}
	st_case_463:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st464
		}
		goto st66
	st464:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof464
		}
	st_case_464:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st465
		}
		goto st66
	st465:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof465
		}
	st_case_465:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st466
		}
		goto st66
	st466:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof466
		}
	st_case_466:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st467
		}
		goto st66
	st467:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof467
		}
	st_case_467:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st468
		}
		goto st66
	st468:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof468
		}
	st_case_468:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st469
		}
		goto st66
	st469:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof469
		}
	st_case_469:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st470
		}
		goto st66
	st470:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof470
		}
	st_case_470:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st471
		}
		goto st66
	st471:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof471
		}
	st_case_471:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st472
		}
		goto st66
	st472:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof472
		}
	st_case_472:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st473
		}
		goto st66
	st473:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof473
		}
	st_case_473:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st474
		}
		goto st66
	st474:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof474
		}
	st_case_474:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st475
		}
		goto st66
	st475:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof475
		}
	st_case_475:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st476
		}
		goto st66
	st476:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof476
		}
	st_case_476:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st477
		}
		goto st66
	st477:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof477
		}
	st_case_477:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr705
		case 10:
			goto tr678
		case 11:
			goto tr706
		case 12:
			goto tr572
		case 13:
			goto tr680
		case 32:
			goto tr705
		case 34:
			goto tr208
		case 44:
			goto tr158
		case 61:
			goto tr165
		case 92:
			goto st68
		}
		goto st66
	tr672:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr861:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr732:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr745:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr752:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr759:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr893:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr897:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr902:
		(m.cs) = 107
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st107:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof107
		}
	st_case_107:
//line plugins/parsers/influx/machine.go:16977
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr258
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr279
		}
		goto tr278
	tr278:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st108
	st108:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof108
		}
	st_case_108:
//line plugins/parsers/influx/machine.go:17010
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr261
		case 44:
			goto st6
		case 61:
			goto tr281
		case 92:
			goto st122
		}
		goto st108
	tr281:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st109
	st109:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof109
		}
	st_case_109:
//line plugins/parsers/influx/machine.go:17047
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr266
		case 44:
			goto st6
		case 45:
			goto tr283
		case 46:
			goto tr284
		case 48:
			goto tr285
		case 61:
			goto st6
		case 70:
			goto tr287
		case 84:
			goto tr288
		case 92:
			goto tr153
		case 102:
			goto tr289
		case 116:
			goto tr290
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr286
		}
		goto tr148
	tr283:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st110
	st110:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof110
		}
	st_case_110:
//line plugins/parsers/influx/machine.go:17097
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 46:
			goto st111
		case 48:
			goto st482
		case 61:
			goto st6
		case 92:
			goto st63
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st485
		}
		goto st48
	tr284:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st111
	st111:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof111
		}
	st_case_111:
//line plugins/parsers/influx/machine.go:17139
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st478
		}
		goto st48
	st478:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof478
		}
	st_case_478:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st478
		}
		goto st48
	st112:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof112
		}
	st_case_112:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr295
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		}
		switch {
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st481
			}
		case (m.data)[(m.p)] >= 43:
			goto st113
		}
		goto st48
	tr295:
		(m.cs) = 479
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st479:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof479
		}
	st_case_479:
//line plugins/parsers/influx/machine.go:17255
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr567
		case 13:
			goto st33
		case 32:
			goto tr566
		case 44:
			goto tr568
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st480
			}
		case (m.data)[(m.p)] >= 9:
			goto tr566
		}
		goto st16
	st480:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof480
		}
	st_case_480:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st480
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	st113:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof113
		}
	st_case_113:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st481
		}
		goto st48
	st481:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof481
		}
	st_case_481:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 61:
			goto st6
		case 92:
			goto st63
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st481
		}
		goto st48
	st482:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof482
		}
	st_case_482:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 46:
			goto st478
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		case 105:
			goto st484
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st483
		}
		goto st48
	st483:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof483
		}
	st_case_483:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 46:
			goto st478
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st483
		}
		goto st48
	st484:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof484
		}
	st_case_484:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr740
		case 10:
			goto tr741
		case 11:
			goto tr742
		case 12:
			goto tr743
		case 13:
			goto tr744
		case 32:
			goto tr740
		case 34:
			goto tr157
		case 44:
			goto tr745
		case 61:
			goto st6
		case 92:
			goto st63
		}
		goto st48
	st485:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof485
		}
	st_case_485:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 46:
			goto st478
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		case 105:
			goto st484
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st485
		}
		goto st48
	tr285:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st486
	st486:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof486
		}
	st_case_486:
//line plugins/parsers/influx/machine.go:17527
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 46:
			goto st478
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		case 105:
			goto st484
		case 117:
			goto st487
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st483
		}
		goto st48
	st487:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof487
		}
	st_case_487:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr747
		case 10:
			goto tr748
		case 11:
			goto tr749
		case 12:
			goto tr750
		case 13:
			goto tr751
		case 32:
			goto tr747
		case 34:
			goto tr157
		case 44:
			goto tr752
		case 61:
			goto st6
		case 92:
			goto st63
		}
		goto st48
	tr286:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st488
	st488:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof488
		}
	st_case_488:
//line plugins/parsers/influx/machine.go:17603
		switch (m.data)[(m.p)] {
		case 9:
			goto tr729
		case 10:
			goto tr534
		case 11:
			goto tr730
		case 12:
			goto tr731
		case 13:
			goto tr536
		case 32:
			goto tr729
		case 34:
			goto tr157
		case 44:
			goto tr732
		case 46:
			goto st478
		case 61:
			goto st6
		case 69:
			goto st112
		case 92:
			goto st63
		case 101:
			goto st112
		case 105:
			goto st484
		case 117:
			goto st487
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st488
		}
		goto st48
	tr287:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st489
	st489:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof489
		}
	st_case_489:
//line plugins/parsers/influx/machine.go:17651
		switch (m.data)[(m.p)] {
		case 9:
			goto tr754
		case 10:
			goto tr755
		case 11:
			goto tr756
		case 12:
			goto tr757
		case 13:
			goto tr758
		case 32:
			goto tr754
		case 34:
			goto tr157
		case 44:
			goto tr759
		case 61:
			goto st6
		case 65:
			goto st114
		case 92:
			goto st63
		case 97:
			goto st117
		}
		goto st48
	st114:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof114
		}
	st_case_114:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 76:
			goto st115
		case 92:
			goto st63
		}
		goto st48
	st115:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof115
		}
	st_case_115:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 83:
			goto st116
		case 92:
			goto st63
		}
		goto st48
	st116:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof116
		}
	st_case_116:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 69:
			goto st490
		case 92:
			goto st63
		}
		goto st48
	st490:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof490
		}
	st_case_490:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr754
		case 10:
			goto tr755
		case 11:
			goto tr756
		case 12:
			goto tr757
		case 13:
			goto tr758
		case 32:
			goto tr754
		case 34:
			goto tr157
		case 44:
			goto tr759
		case 61:
			goto st6
		case 92:
			goto st63
		}
		goto st48
	st117:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof117
		}
	st_case_117:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		case 108:
			goto st118
		}
		goto st48
	st118:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof118
		}
	st_case_118:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		case 115:
			goto st119
		}
		goto st48
	st119:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof119
		}
	st_case_119:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		case 101:
			goto st490
		}
		goto st48
	tr288:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st491
	st491:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof491
		}
	st_case_491:
//line plugins/parsers/influx/machine.go:17898
		switch (m.data)[(m.p)] {
		case 9:
			goto tr754
		case 10:
			goto tr755
		case 11:
			goto tr756
		case 12:
			goto tr757
		case 13:
			goto tr758
		case 32:
			goto tr754
		case 34:
			goto tr157
		case 44:
			goto tr759
		case 61:
			goto st6
		case 82:
			goto st120
		case 92:
			goto st63
		case 114:
			goto st121
		}
		goto st48
	st120:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof120
		}
	st_case_120:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 85:
			goto st116
		case 92:
			goto st63
		}
		goto st48
	st121:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof121
		}
	st_case_121:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr155
		case 10:
			goto tr29
		case 11:
			goto tr156
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr155
		case 34:
			goto tr157
		case 44:
			goto tr158
		case 61:
			goto st6
		case 92:
			goto st63
		case 117:
			goto st119
		}
		goto st48
	tr289:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st492
	st492:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof492
		}
	st_case_492:
//line plugins/parsers/influx/machine.go:17997
		switch (m.data)[(m.p)] {
		case 9:
			goto tr754
		case 10:
			goto tr755
		case 11:
			goto tr756
		case 12:
			goto tr757
		case 13:
			goto tr758
		case 32:
			goto tr754
		case 34:
			goto tr157
		case 44:
			goto tr759
		case 61:
			goto st6
		case 92:
			goto st63
		case 97:
			goto st117
		}
		goto st48
	tr290:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st493
	st493:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof493
		}
	st_case_493:
//line plugins/parsers/influx/machine.go:18034
		switch (m.data)[(m.p)] {
		case 9:
			goto tr754
		case 10:
			goto tr755
		case 11:
			goto tr756
		case 12:
			goto tr757
		case 13:
			goto tr758
		case 32:
			goto tr754
		case 34:
			goto tr157
		case 44:
			goto tr759
		case 61:
			goto st6
		case 92:
			goto st63
		case 114:
			goto st121
		}
		goto st48
	tr279:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st122
	st122:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof122
		}
	st_case_122:
//line plugins/parsers/influx/machine.go:18071
		switch (m.data)[(m.p)] {
		case 34:
			goto st108
		case 92:
			goto st123
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st45
	st123:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof123
		}
	st_case_123:
//line plugins/parsers/influx/machine.go:18095
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr261
		case 44:
			goto st6
		case 61:
			goto tr281
		case 92:
			goto st122
		}
		goto st108
	tr267:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st124
	st124:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof124
		}
	st_case_124:
//line plugins/parsers/influx/machine.go:18128
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 46:
			goto st125
		case 48:
			goto st518
		case 61:
			goto st6
		case 92:
			goto st86
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st521
		}
		goto st80
	tr268:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st125
	st125:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof125
		}
	st_case_125:
//line plugins/parsers/influx/machine.go:18170
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st494
		}
		goto st80
	st494:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof494
		}
	st_case_494:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st494
		}
		goto st80
	tr766:
		(m.cs) = 495
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr799:
		(m.cs) = 495
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr805:
		(m.cs) = 495
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr811:
		(m.cs) = 495
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st495:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof495
		}
	st_case_495:
//line plugins/parsers/influx/machine.go:18329
		switch (m.data)[(m.p)] {
		case 9:
			goto tr769
		case 10:
			goto tr221
		case 11:
			goto tr770
		case 12:
			goto tr566
		case 13:
			goto st73
		case 32:
			goto tr769
		case 34:
			goto tr204
		case 44:
			goto tr233
		case 45:
			goto tr771
		case 61:
			goto st6
		case 92:
			goto tr237
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr772
		}
		goto tr235
	tr770:
		(m.cs) = 496
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st496:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof496
		}
	st_case_496:
//line plugins/parsers/influx/machine.go:18380
		switch (m.data)[(m.p)] {
		case 9:
			goto tr769
		case 10:
			goto tr221
		case 11:
			goto tr770
		case 12:
			goto tr566
		case 13:
			goto st73
		case 32:
			goto tr769
		case 34:
			goto tr204
		case 44:
			goto tr233
		case 45:
			goto tr771
		case 61:
			goto tr101
		case 92:
			goto tr237
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr772
		}
		goto tr235
	tr771:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st126
	st126:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof126
		}
	st_case_126:
//line plugins/parsers/influx/machine.go:18420
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr239
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st497
		}
		goto st82
	tr772:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st497
	st497:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof497
		}
	st_case_497:
//line plugins/parsers/influx/machine.go:18458
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st499
		}
		goto st82
	tr777:
		(m.cs) = 498
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr774:
		(m.cs) = 498
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st498:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof498
		}
	st_case_498:
//line plugins/parsers/influx/machine.go:18530
		switch (m.data)[(m.p)] {
		case 9:
			goto tr776
		case 10:
			goto tr221
		case 11:
			goto tr777
		case 12:
			goto tr575
		case 13:
			goto st73
		case 32:
			goto tr776
		case 34:
			goto tr204
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto tr237
		}
		goto tr235
	st499:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof499
		}
	st_case_499:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st500
		}
		goto st82
	st500:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof500
		}
	st_case_500:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st501
		}
		goto st82
	st501:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof501
		}
	st_case_501:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st502
		}
		goto st82
	st502:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof502
		}
	st_case_502:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st503
		}
		goto st82
	st503:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof503
		}
	st_case_503:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st504
		}
		goto st82
	st504:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof504
		}
	st_case_504:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st505
		}
		goto st82
	st505:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof505
		}
	st_case_505:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st506
		}
		goto st82
	st506:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof506
		}
	st_case_506:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st507
		}
		goto st82
	st507:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof507
		}
	st_case_507:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st508
		}
		goto st82
	st508:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof508
		}
	st_case_508:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st509
		}
		goto st82
	st509:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof509
		}
	st_case_509:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st510
		}
		goto st82
	st510:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof510
		}
	st_case_510:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st511
		}
		goto st82
	st511:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof511
		}
	st_case_511:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st512
		}
		goto st82
	st512:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof512
		}
	st_case_512:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st513
		}
		goto st82
	st513:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof513
		}
	st_case_513:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st514
		}
		goto st82
	st514:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof514
		}
	st_case_514:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st515
		}
		goto st82
	st515:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof515
		}
	st_case_515:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st516
		}
		goto st82
	st516:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof516
		}
	st_case_516:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr773
		case 10:
			goto tr603
		case 11:
			goto tr774
		case 12:
			goto tr572
		case 13:
			goto tr605
		case 32:
			goto tr773
		case 34:
			goto tr208
		case 44:
			goto tr233
		case 61:
			goto tr101
		case 92:
			goto st84
		}
		goto st82
	st127:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof127
		}
	st_case_127:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr295
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		}
		switch {
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st517
			}
		case (m.data)[(m.p)] >= 43:
			goto st128
		}
		goto st80
	st128:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof128
		}
	st_case_128:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st517
		}
		goto st80
	st517:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof517
		}
	st_case_517:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 61:
			goto st6
		case 92:
			goto st86
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st517
		}
		goto st80
	st518:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof518
		}
	st_case_518:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 46:
			goto st494
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		case 105:
			goto st520
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st519
		}
		goto st80
	st519:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof519
		}
	st_case_519:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 46:
			goto st494
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st519
		}
		goto st80
	st520:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof520
		}
	st_case_520:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr797
		case 10:
			goto tr798
		case 11:
			goto tr799
		case 12:
			goto tr743
		case 13:
			goto tr800
		case 32:
			goto tr797
		case 34:
			goto tr157
		case 44:
			goto tr801
		case 61:
			goto st6
		case 92:
			goto st86
		}
		goto st80
	st521:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof521
		}
	st_case_521:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 46:
			goto st494
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		case 105:
			goto st520
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st521
		}
		goto st80
	tr269:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st522
	st522:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof522
		}
	st_case_522:
//line plugins/parsers/influx/machine.go:19361
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 46:
			goto st494
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		case 105:
			goto st520
		case 117:
			goto st523
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st519
		}
		goto st80
	st523:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof523
		}
	st_case_523:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr803
		case 10:
			goto tr804
		case 11:
			goto tr805
		case 12:
			goto tr750
		case 13:
			goto tr806
		case 32:
			goto tr803
		case 34:
			goto tr157
		case 44:
			goto tr807
		case 61:
			goto st6
		case 92:
			goto st86
		}
		goto st80
	tr270:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st524
	st524:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof524
		}
	st_case_524:
//line plugins/parsers/influx/machine.go:19437
		switch (m.data)[(m.p)] {
		case 9:
			goto tr764
		case 10:
			goto tr765
		case 11:
			goto tr766
		case 12:
			goto tr731
		case 13:
			goto tr642
		case 32:
			goto tr764
		case 34:
			goto tr157
		case 44:
			goto tr767
		case 46:
			goto st494
		case 61:
			goto st6
		case 69:
			goto st127
		case 92:
			goto st86
		case 101:
			goto st127
		case 105:
			goto st520
		case 117:
			goto st523
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st524
		}
		goto st80
	tr271:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st525
	st525:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof525
		}
	st_case_525:
//line plugins/parsers/influx/machine.go:19485
		switch (m.data)[(m.p)] {
		case 9:
			goto tr809
		case 10:
			goto tr810
		case 11:
			goto tr811
		case 12:
			goto tr757
		case 13:
			goto tr812
		case 32:
			goto tr809
		case 34:
			goto tr157
		case 44:
			goto tr813
		case 61:
			goto st6
		case 65:
			goto st129
		case 92:
			goto st86
		case 97:
			goto st132
		}
		goto st80
	st129:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof129
		}
	st_case_129:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 76:
			goto st130
		case 92:
			goto st86
		}
		goto st80
	st130:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof130
		}
	st_case_130:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 83:
			goto st131
		case 92:
			goto st86
		}
		goto st80
	st131:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof131
		}
	st_case_131:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 69:
			goto st526
		case 92:
			goto st86
		}
		goto st80
	st526:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof526
		}
	st_case_526:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr809
		case 10:
			goto tr810
		case 11:
			goto tr811
		case 12:
			goto tr757
		case 13:
			goto tr812
		case 32:
			goto tr809
		case 34:
			goto tr157
		case 44:
			goto tr813
		case 61:
			goto st6
		case 92:
			goto st86
		}
		goto st80
	st132:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof132
		}
	st_case_132:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		case 108:
			goto st133
		}
		goto st80
	st133:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof133
		}
	st_case_133:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		case 115:
			goto st134
		}
		goto st80
	st134:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof134
		}
	st_case_134:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		case 101:
			goto st526
		}
		goto st80
	tr272:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st527
	st527:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof527
		}
	st_case_527:
//line plugins/parsers/influx/machine.go:19732
		switch (m.data)[(m.p)] {
		case 9:
			goto tr809
		case 10:
			goto tr810
		case 11:
			goto tr811
		case 12:
			goto tr757
		case 13:
			goto tr812
		case 32:
			goto tr809
		case 34:
			goto tr157
		case 44:
			goto tr813
		case 61:
			goto st6
		case 82:
			goto st135
		case 92:
			goto st86
		case 114:
			goto st136
		}
		goto st80
	st135:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof135
		}
	st_case_135:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 85:
			goto st131
		case 92:
			goto st86
		}
		goto st80
	st136:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof136
		}
	st_case_136:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr231
		case 10:
			goto tr29
		case 11:
			goto tr232
		case 12:
			goto tr60
		case 13:
			goto st7
		case 32:
			goto tr231
		case 34:
			goto tr157
		case 44:
			goto tr233
		case 61:
			goto st6
		case 92:
			goto st86
		case 117:
			goto st134
		}
		goto st80
	tr273:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st528
	st528:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof528
		}
	st_case_528:
//line plugins/parsers/influx/machine.go:19831
		switch (m.data)[(m.p)] {
		case 9:
			goto tr809
		case 10:
			goto tr810
		case 11:
			goto tr811
		case 12:
			goto tr757
		case 13:
			goto tr812
		case 32:
			goto tr809
		case 34:
			goto tr157
		case 44:
			goto tr813
		case 61:
			goto st6
		case 92:
			goto st86
		case 97:
			goto st132
		}
		goto st80
	tr274:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st529
	st529:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof529
		}
	st_case_529:
//line plugins/parsers/influx/machine.go:19868
		switch (m.data)[(m.p)] {
		case 9:
			goto tr809
		case 10:
			goto tr810
		case 11:
			goto tr811
		case 12:
			goto tr757
		case 13:
			goto tr812
		case 32:
			goto tr809
		case 34:
			goto tr157
		case 44:
			goto tr813
		case 61:
			goto st6
		case 92:
			goto st86
		case 114:
			goto st136
		}
		goto st80
	tr259:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st137
	st137:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof137
		}
	st_case_137:
//line plugins/parsers/influx/machine.go:19905
		switch (m.data)[(m.p)] {
		case 34:
			goto st98
		case 92:
			goto st138
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr47
			}
		case (m.data)[(m.p)] >= 9:
			goto tr47
		}
		goto st45
	st138:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof138
		}
	st_case_138:
//line plugins/parsers/influx/machine.go:19929
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr47
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr261
		case 44:
			goto st6
		case 61:
			goto tr262
		case 92:
			goto st137
		}
		goto st98
	st139:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof139
		}
	st_case_139:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr317
		case 44:
			goto tr92
		case 92:
			goto st141
		}
		switch {
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st532
			}
		case (m.data)[(m.p)] >= 43:
			goto st140
		}
		goto st30
	tr317:
		(m.cs) = 530
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st530:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof530
		}
	st_case_530:
//line plugins/parsers/influx/machine.go:20003
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 11:
			goto tr637
		case 13:
			goto st33
		case 32:
			goto tr501
		case 44:
			goto tr503
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st531
			}
		case (m.data)[(m.p)] >= 9:
			goto tr501
		}
		goto st1
	st531:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof531
		}
	st_case_531:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st531
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	st140:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof140
		}
	st_case_140:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st532
		}
		goto st30
	st532:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof532
		}
	st_case_532:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 92:
			goto st141
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st532
		}
		goto st30
	tr87:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st141
	st141:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof141
		}
	st_case_141:
//line plugins/parsers/influx/machine.go:20124
		switch (m.data)[(m.p)] {
		case 34:
			goto st30
		case 92:
			goto st30
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st1
	st533:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof533
		}
	st_case_533:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 46:
			goto st407
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		case 105:
			goto st535
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st534
		}
		goto st30
	st534:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof534
		}
	st_case_534:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 46:
			goto st407
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st534
		}
		goto st30
	st535:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof535
		}
	st_case_535:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr822
		case 10:
			goto tr823
		case 11:
			goto tr824
		case 12:
			goto tr825
		case 13:
			goto tr800
		case 32:
			goto tr822
		case 34:
			goto tr91
		case 44:
			goto tr826
		case 92:
			goto st141
		}
		goto st30
	st536:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof536
		}
	st_case_536:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 46:
			goto st407
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		case 105:
			goto st535
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st536
		}
		goto st30
	tr247:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st537
	st537:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof537
		}
	st_case_537:
//line plugins/parsers/influx/machine.go:20286
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 46:
			goto st407
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		case 105:
			goto st535
		case 117:
			goto st538
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st534
		}
		goto st30
	st538:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof538
		}
	st_case_538:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr828
		case 10:
			goto tr829
		case 11:
			goto tr830
		case 12:
			goto tr831
		case 13:
			goto tr806
		case 32:
			goto tr828
		case 34:
			goto tr91
		case 44:
			goto tr832
		case 92:
			goto st141
		}
		goto st30
	tr248:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st539
	st539:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof539
		}
	st_case_539:
//line plugins/parsers/influx/machine.go:20358
		switch (m.data)[(m.p)] {
		case 9:
			goto tr638
		case 10:
			goto tr639
		case 11:
			goto tr640
		case 12:
			goto tr641
		case 13:
			goto tr642
		case 32:
			goto tr638
		case 34:
			goto tr91
		case 44:
			goto tr643
		case 46:
			goto st407
		case 69:
			goto st139
		case 92:
			goto st141
		case 101:
			goto st139
		case 105:
			goto st535
		case 117:
			goto st538
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st539
		}
		goto st30
	tr249:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st540
	st540:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof540
		}
	st_case_540:
//line plugins/parsers/influx/machine.go:20404
		switch (m.data)[(m.p)] {
		case 9:
			goto tr834
		case 10:
			goto tr810
		case 11:
			goto tr835
		case 12:
			goto tr836
		case 13:
			goto tr812
		case 32:
			goto tr834
		case 34:
			goto tr91
		case 44:
			goto tr837
		case 65:
			goto st142
		case 92:
			goto st141
		case 97:
			goto st145
		}
		goto st30
	st142:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof142
		}
	st_case_142:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 76:
			goto st143
		case 92:
			goto st141
		}
		goto st30
	st143:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof143
		}
	st_case_143:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 83:
			goto st144
		case 92:
			goto st141
		}
		goto st30
	st144:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof144
		}
	st_case_144:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 69:
			goto st541
		case 92:
			goto st141
		}
		goto st30
	st541:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof541
		}
	st_case_541:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr834
		case 10:
			goto tr810
		case 11:
			goto tr835
		case 12:
			goto tr836
		case 13:
			goto tr812
		case 32:
			goto tr834
		case 34:
			goto tr91
		case 44:
			goto tr837
		case 92:
			goto st141
		}
		goto st30
	st145:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof145
		}
	st_case_145:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		case 108:
			goto st146
		}
		goto st30
	st146:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof146
		}
	st_case_146:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		case 115:
			goto st147
		}
		goto st30
	st147:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof147
		}
	st_case_147:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		case 101:
			goto st541
		}
		goto st30
	tr250:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st542
	st542:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof542
		}
	st_case_542:
//line plugins/parsers/influx/machine.go:20635
		switch (m.data)[(m.p)] {
		case 9:
			goto tr834
		case 10:
			goto tr810
		case 11:
			goto tr835
		case 12:
			goto tr836
		case 13:
			goto tr812
		case 32:
			goto tr834
		case 34:
			goto tr91
		case 44:
			goto tr837
		case 82:
			goto st148
		case 92:
			goto st141
		case 114:
			goto st149
		}
		goto st30
	st148:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof148
		}
	st_case_148:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 85:
			goto st144
		case 92:
			goto st141
		}
		goto st30
	st149:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof149
		}
	st_case_149:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr89
		case 10:
			goto tr29
		case 11:
			goto tr90
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr89
		case 34:
			goto tr91
		case 44:
			goto tr92
		case 92:
			goto st141
		case 117:
			goto st147
		}
		goto st30
	tr251:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st543
	st543:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof543
		}
	st_case_543:
//line plugins/parsers/influx/machine.go:20728
		switch (m.data)[(m.p)] {
		case 9:
			goto tr834
		case 10:
			goto tr810
		case 11:
			goto tr835
		case 12:
			goto tr836
		case 13:
			goto tr812
		case 32:
			goto tr834
		case 34:
			goto tr91
		case 44:
			goto tr837
		case 92:
			goto st141
		case 97:
			goto st145
		}
		goto st30
	tr252:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st544
	st544:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof544
		}
	st_case_544:
//line plugins/parsers/influx/machine.go:20763
		switch (m.data)[(m.p)] {
		case 9:
			goto tr834
		case 10:
			goto tr810
		case 11:
			goto tr835
		case 12:
			goto tr836
		case 13:
			goto tr812
		case 32:
			goto tr834
		case 34:
			goto tr91
		case 44:
			goto tr837
		case 92:
			goto st141
		case 114:
			goto st149
		}
		goto st30
	st545:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof545
		}
	st_case_545:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st546
		}
		goto st41
	st546:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof546
		}
	st_case_546:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st547
		}
		goto st41
	st547:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof547
		}
	st_case_547:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st548
		}
		goto st41
	st548:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof548
		}
	st_case_548:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st549
		}
		goto st41
	st549:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof549
		}
	st_case_549:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st550
		}
		goto st41
	st550:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof550
		}
	st_case_550:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st551
		}
		goto st41
	st551:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof551
		}
	st_case_551:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st552
		}
		goto st41
	st552:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof552
		}
	st_case_552:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st553
		}
		goto st41
	st553:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof553
		}
	st_case_553:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st554
		}
		goto st41
	st554:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof554
		}
	st_case_554:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st555
		}
		goto st41
	st555:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof555
		}
	st_case_555:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st556
		}
		goto st41
	st556:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof556
		}
	st_case_556:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st557
		}
		goto st41
	st557:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof557
		}
	st_case_557:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st558
		}
		goto st41
	st558:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof558
		}
	st_case_558:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st559
		}
		goto st41
	st559:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof559
		}
	st_case_559:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st560
		}
		goto st41
	st560:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof560
		}
	st_case_560:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st561
		}
		goto st41
	st561:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof561
		}
	st_case_561:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st562
		}
		goto st41
	st562:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof562
		}
	st_case_562:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr630
		case 10:
			goto tr603
		case 11:
			goto tr631
		case 12:
			goto tr509
		case 13:
			goto tr605
		case 32:
			goto tr630
		case 34:
			goto tr128
		case 44:
			goto tr92
		case 61:
			goto tr129
		case 92:
			goto st93
		}
		goto st41
	tr213:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st150
	st150:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof150
		}
	st_case_150:
//line plugins/parsers/influx/machine.go:21353
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 46:
			goto st151
		case 48:
			goto st587
		case 92:
			goto st156
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st590
		}
		goto st54
	tr214:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st151
	st151:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof151
		}
	st_case_151:
//line plugins/parsers/influx/machine.go:21393
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st563
		}
		goto st54
	st563:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof563
		}
	st_case_563:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st563
		}
		goto st54
	tr860:
		(m.cs) = 564
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr892:
		(m.cs) = 564
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr896:
		(m.cs) = 564
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr901:
		(m.cs) = 564
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st564:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof564
		}
	st_case_564:
//line plugins/parsers/influx/machine.go:21548
		switch (m.data)[(m.p)] {
		case 9:
			goto tr863
		case 10:
			goto tr275
		case 11:
			goto tr864
		case 12:
			goto tr501
		case 13:
			goto st103
		case 32:
			goto tr863
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 45:
			goto tr865
		case 61:
			goto st54
		case 92:
			goto tr186
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr866
		}
		goto tr184
	tr864:
		(m.cs) = 565
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	st565:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof565
		}
	st_case_565:
//line plugins/parsers/influx/machine.go:21599
		switch (m.data)[(m.p)] {
		case 9:
			goto tr863
		case 10:
			goto tr275
		case 11:
			goto tr864
		case 12:
			goto tr501
		case 13:
			goto st103
		case 32:
			goto tr863
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 45:
			goto tr865
		case 61:
			goto tr189
		case 92:
			goto tr186
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr866
		}
		goto tr184
	tr865:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st152
	st152:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof152
		}
	st_case_152:
//line plugins/parsers/influx/machine.go:21639
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr188
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st566
		}
		goto st56
	tr866:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st566
	st566:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof566
		}
	st_case_566:
//line plugins/parsers/influx/machine.go:21677
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st568
		}
		goto st56
	tr871:
		(m.cs) = 567
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto _again
	tr868:
		(m.cs) = 567
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:149

		err = m.handler.SetTimestamp(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st567:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof567
		}
	st_case_567:
//line plugins/parsers/influx/machine.go:21749
		switch (m.data)[(m.p)] {
		case 9:
			goto tr870
		case 10:
			goto tr275
		case 11:
			goto tr871
		case 12:
			goto tr514
		case 13:
			goto st103
		case 32:
			goto tr870
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto tr186
		}
		goto tr184
	tr186:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st153
	st153:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof153
		}
	st_case_153:
//line plugins/parsers/influx/machine.go:21784
		switch (m.data)[(m.p)] {
		case 34:
			goto st56
		case 92:
			goto st56
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st11
	st568:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof568
		}
	st_case_568:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st569
		}
		goto st56
	st569:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof569
		}
	st_case_569:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st570
		}
		goto st56
	st570:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof570
		}
	st_case_570:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st571
		}
		goto st56
	st571:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof571
		}
	st_case_571:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st572
		}
		goto st56
	st572:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof572
		}
	st_case_572:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st573
		}
		goto st56
	st573:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof573
		}
	st_case_573:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st574
		}
		goto st56
	st574:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof574
		}
	st_case_574:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st575
		}
		goto st56
	st575:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof575
		}
	st_case_575:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st576
		}
		goto st56
	st576:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof576
		}
	st_case_576:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st577
		}
		goto st56
	st577:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof577
		}
	st_case_577:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st578
		}
		goto st56
	st578:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof578
		}
	st_case_578:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st579
		}
		goto st56
	st579:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof579
		}
	st_case_579:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st580
		}
		goto st56
	st580:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof580
		}
	st_case_580:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st581
		}
		goto st56
	st581:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof581
		}
	st_case_581:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st582
		}
		goto st56
	st582:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof582
		}
	st_case_582:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st583
		}
		goto st56
	st583:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof583
		}
	st_case_583:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st584
		}
		goto st56
	st584:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof584
		}
	st_case_584:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st585
		}
		goto st56
	st585:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof585
		}
	st_case_585:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr867
		case 10:
			goto tr678
		case 11:
			goto tr868
		case 12:
			goto tr509
		case 13:
			goto tr680
		case 32:
			goto tr867
		case 34:
			goto tr128
		case 44:
			goto tr182
		case 61:
			goto tr189
		case 92:
			goto st153
		}
		goto st56
	st154:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof154
		}
	st_case_154:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr317
		case 44:
			goto tr182
		case 92:
			goto st156
		}
		switch {
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st586
			}
		case (m.data)[(m.p)] >= 43:
			goto st155
		}
		goto st54
	st155:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof155
		}
	st_case_155:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st586
		}
		goto st54
	st586:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof586
		}
	st_case_586:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 92:
			goto st156
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st586
		}
		goto st54
	tr340:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st156
	st156:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof156
		}
	st_case_156:
//line plugins/parsers/influx/machine.go:22458
		switch (m.data)[(m.p)] {
		case 34:
			goto st54
		case 92:
			goto st54
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st1
	st587:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof587
		}
	st_case_587:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 46:
			goto st563
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		case 105:
			goto st589
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st588
		}
		goto st54
	st588:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof588
		}
	st_case_588:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 46:
			goto st563
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st588
		}
		goto st54
	st589:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof589
		}
	st_case_589:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr891
		case 10:
			goto tr741
		case 11:
			goto tr892
		case 12:
			goto tr825
		case 13:
			goto tr744
		case 32:
			goto tr891
		case 34:
			goto tr91
		case 44:
			goto tr893
		case 92:
			goto st156
		}
		goto st54
	st590:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof590
		}
	st_case_590:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 46:
			goto st563
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		case 105:
			goto st589
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st590
		}
		goto st54
	tr215:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st591
	st591:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof591
		}
	st_case_591:
//line plugins/parsers/influx/machine.go:22620
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 46:
			goto st563
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		case 105:
			goto st589
		case 117:
			goto st592
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st588
		}
		goto st54
	st592:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof592
		}
	st_case_592:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr895
		case 10:
			goto tr748
		case 11:
			goto tr896
		case 12:
			goto tr831
		case 13:
			goto tr751
		case 32:
			goto tr895
		case 34:
			goto tr91
		case 44:
			goto tr897
		case 92:
			goto st156
		}
		goto st54
	tr216:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st593
	st593:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof593
		}
	st_case_593:
//line plugins/parsers/influx/machine.go:22692
		switch (m.data)[(m.p)] {
		case 9:
			goto tr859
		case 10:
			goto tr534
		case 11:
			goto tr860
		case 12:
			goto tr641
		case 13:
			goto tr536
		case 32:
			goto tr859
		case 34:
			goto tr91
		case 44:
			goto tr861
		case 46:
			goto st563
		case 69:
			goto st154
		case 92:
			goto st156
		case 101:
			goto st154
		case 105:
			goto st589
		case 117:
			goto st592
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st593
		}
		goto st54
	tr217:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st594
	st594:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof594
		}
	st_case_594:
//line plugins/parsers/influx/machine.go:22738
		switch (m.data)[(m.p)] {
		case 9:
			goto tr899
		case 10:
			goto tr900
		case 11:
			goto tr901
		case 12:
			goto tr836
		case 13:
			goto tr758
		case 32:
			goto tr899
		case 34:
			goto tr91
		case 44:
			goto tr902
		case 65:
			goto st157
		case 92:
			goto st156
		case 97:
			goto st160
		}
		goto st54
	st157:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof157
		}
	st_case_157:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 76:
			goto st158
		case 92:
			goto st156
		}
		goto st54
	st158:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof158
		}
	st_case_158:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 83:
			goto st159
		case 92:
			goto st156
		}
		goto st54
	st159:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof159
		}
	st_case_159:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 69:
			goto st595
		case 92:
			goto st156
		}
		goto st54
	st595:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof595
		}
	st_case_595:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr899
		case 10:
			goto tr900
		case 11:
			goto tr901
		case 12:
			goto tr836
		case 13:
			goto tr758
		case 32:
			goto tr899
		case 34:
			goto tr91
		case 44:
			goto tr902
		case 92:
			goto st156
		}
		goto st54
	st160:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof160
		}
	st_case_160:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		case 108:
			goto st161
		}
		goto st54
	st161:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof161
		}
	st_case_161:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		case 115:
			goto st162
		}
		goto st54
	st162:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof162
		}
	st_case_162:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		case 101:
			goto st595
		}
		goto st54
	tr218:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st596
	st596:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof596
		}
	st_case_596:
//line plugins/parsers/influx/machine.go:22969
		switch (m.data)[(m.p)] {
		case 9:
			goto tr899
		case 10:
			goto tr900
		case 11:
			goto tr901
		case 12:
			goto tr836
		case 13:
			goto tr758
		case 32:
			goto tr899
		case 34:
			goto tr91
		case 44:
			goto tr902
		case 82:
			goto st163
		case 92:
			goto st156
		case 114:
			goto st164
		}
		goto st54
	st163:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof163
		}
	st_case_163:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 85:
			goto st159
		case 92:
			goto st156
		}
		goto st54
	st164:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof164
		}
	st_case_164:
		switch (m.data)[(m.p)] {
		case 9:
			goto tr180
		case 10:
			goto tr29
		case 11:
			goto tr181
		case 12:
			goto tr1
		case 13:
			goto st7
		case 32:
			goto tr180
		case 34:
			goto tr91
		case 44:
			goto tr182
		case 92:
			goto st156
		case 117:
			goto st162
		}
		goto st54
	tr219:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st597
	st597:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof597
		}
	st_case_597:
//line plugins/parsers/influx/machine.go:23062
		switch (m.data)[(m.p)] {
		case 9:
			goto tr899
		case 10:
			goto tr900
		case 11:
			goto tr901
		case 12:
			goto tr836
		case 13:
			goto tr758
		case 32:
			goto tr899
		case 34:
			goto tr91
		case 44:
			goto tr902
		case 92:
			goto st156
		case 97:
			goto st160
		}
		goto st54
	tr220:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st598
	st598:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof598
		}
	st_case_598:
//line plugins/parsers/influx/machine.go:23097
		switch (m.data)[(m.p)] {
		case 9:
			goto tr899
		case 10:
			goto tr900
		case 11:
			goto tr901
		case 12:
			goto tr836
		case 13:
			goto tr758
		case 32:
			goto tr899
		case 34:
			goto tr91
		case 44:
			goto tr902
		case 92:
			goto st156
		case 114:
			goto st164
		}
		goto st54
	st165:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof165
		}
	st_case_165:
		switch (m.data)[(m.p)] {
		case 9:
			goto st165
		case 10:
			goto tr29
		case 11:
			goto tr339
		case 12:
			goto st8
		case 13:
			goto st7
		case 32:
			goto st165
		case 34:
			goto tr118
		case 35:
			goto st6
		case 44:
			goto st6
		case 92:
			goto tr340
		}
		goto tr337
	tr339:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st166
	st166:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof166
		}
	st_case_166:
//line plugins/parsers/influx/machine.go:23160
		switch (m.data)[(m.p)] {
		case 9:
			goto tr341
		case 10:
			goto tr29
		case 11:
			goto tr342
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr341
		case 34:
			goto tr85
		case 35:
			goto st54
		case 44:
			goto tr182
		case 92:
			goto tr340
		}
		goto tr337
	tr341:
		(m.cs) = 167
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st167:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof167
		}
	st_case_167:
//line plugins/parsers/influx/machine.go:23202
		switch (m.data)[(m.p)] {
		case 9:
			goto st167
		case 10:
			goto tr29
		case 11:
			goto tr344
		case 12:
			goto st10
		case 13:
			goto st7
		case 32:
			goto st167
		case 34:
			goto tr124
		case 35:
			goto tr160
		case 44:
			goto st6
		case 61:
			goto tr337
		case 92:
			goto tr186
		}
		goto tr184
	tr344:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st168
	tr345:
		(m.cs) = 168
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st168:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof168
		}
	st_case_168:
//line plugins/parsers/influx/machine.go:23256
		switch (m.data)[(m.p)] {
		case 9:
			goto tr341
		case 10:
			goto tr29
		case 11:
			goto tr345
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr341
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 61:
			goto tr346
		case 92:
			goto tr186
		}
		goto tr184
	tr342:
		(m.cs) = 169
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st169:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof169
		}
	st_case_169:
//line plugins/parsers/influx/machine.go:23302
		switch (m.data)[(m.p)] {
		case 9:
			goto tr341
		case 10:
			goto tr29
		case 11:
			goto tr345
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr341
		case 34:
			goto tr124
		case 44:
			goto tr182
		case 61:
			goto tr337
		case 92:
			goto tr186
		}
		goto tr184
	tr541:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st170
	st170:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof170
		}
	st_case_170:
//line plugins/parsers/influx/machine.go:23337
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr105
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st599
		}
		goto st6
	tr542:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st599
	st599:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof599
		}
	st_case_599:
//line plugins/parsers/influx/machine.go:23365
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st600
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st600:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof600
		}
	st_case_600:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st601
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st601:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof601
		}
	st_case_601:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st602
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st602:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof602
		}
	st_case_602:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st603
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st603:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof603
		}
	st_case_603:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st604
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st604:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof604
		}
	st_case_604:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st605
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st605:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof605
		}
	st_case_605:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st606
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st606:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof606
		}
	st_case_606:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st607
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st607:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof607
		}
	st_case_607:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st608
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st608:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof608
		}
	st_case_608:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st609
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st609:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof609
		}
	st_case_609:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st610
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st610:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof610
		}
	st_case_610:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st611
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st611:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof611
		}
	st_case_611:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st612
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st612:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof612
		}
	st_case_612:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st613
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st613:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof613
		}
	st_case_613:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st614
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st614:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof614
		}
	st_case_614:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st615
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st615:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof615
		}
	st_case_615:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st616
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st616:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof616
		}
	st_case_616:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st617
			}
		case (m.data)[(m.p)] >= 9:
			goto tr677
		}
		goto st6
	st617:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof617
		}
	st_case_617:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr678
		case 12:
			goto tr469
		case 13:
			goto tr680
		case 32:
			goto tr677
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr677
		}
		goto st6
	tr926:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st171
	tr537:
		(m.cs) = 171
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr933:
		(m.cs) = 171
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr936:
		(m.cs) = 171
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr940:
		(m.cs) = 171
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st171:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof171
		}
	st_case_171:
//line plugins/parsers/influx/machine.go:23951
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr349
		}
		goto tr348
	tr348:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st172
	st172:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof172
		}
	st_case_172:
//line plugins/parsers/influx/machine.go:23984
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr351
		case 92:
			goto st184
		}
		goto st172
	tr351:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st173
	st173:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof173
		}
	st_case_173:
//line plugins/parsers/influx/machine.go:24017
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr353
		case 45:
			goto tr167
		case 46:
			goto tr168
		case 48:
			goto tr169
		case 70:
			goto tr354
		case 84:
			goto tr355
		case 92:
			goto st75
		case 102:
			goto tr356
		case 116:
			goto tr357
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr170
		}
		goto st6
	tr353:
		(m.cs) = 618
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st618:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof618
		}
	st_case_618:
//line plugins/parsers/influx/machine.go:24066
		switch (m.data)[(m.p)] {
		case 10:
			goto tr669
		case 12:
			goto st272
		case 13:
			goto tr671
		case 32:
			goto tr925
		case 34:
			goto tr26
		case 44:
			goto tr926
		case 92:
			goto tr27
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr925
		}
		goto tr23
	tr169:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st619
	st619:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof619
		}
	st_case_619:
//line plugins/parsers/influx/machine.go:24098
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 46:
			goto st326
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		case 105:
			goto st624
		case 117:
			goto st625
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st620
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	st620:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof620
		}
	st_case_620:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 46:
			goto st326
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st620
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	st174:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof174
		}
	st_case_174:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr358
		case 43:
			goto st175
		case 45:
			goto st175
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st623
		}
		goto st6
	tr358:
		(m.cs) = 621
//line plugins/parsers/influx/machine.go.rl:140

		err = m.handler.AddString(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st621:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof621
		}
	st_case_621:
//line plugins/parsers/influx/machine.go:24213
		switch (m.data)[(m.p)] {
		case 10:
			goto tr103
		case 13:
			goto st33
		case 32:
			goto st272
		case 44:
			goto st36
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st622
			}
		case (m.data)[(m.p)] >= 9:
			goto st272
		}
		goto tr105
	st622:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof622
		}
	st_case_622:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st622
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	st175:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof175
		}
	st_case_175:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st623
		}
		goto st6
	st623:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof623
		}
	st_case_623:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st623
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	st624:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof624
		}
	st_case_624:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr741
		case 12:
			goto tr932
		case 13:
			goto tr744
		case 32:
			goto tr931
		case 34:
			goto tr31
		case 44:
			goto tr933
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr931
		}
		goto st6
	st625:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof625
		}
	st_case_625:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr748
		case 12:
			goto tr935
		case 13:
			goto tr751
		case 32:
			goto tr934
		case 34:
			goto tr31
		case 44:
			goto tr936
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr934
		}
		goto st6
	tr170:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st626
	st626:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof626
		}
	st_case_626:
//line plugins/parsers/influx/machine.go:24369
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 46:
			goto st326
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		case 105:
			goto st624
		case 117:
			goto st625
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st626
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	tr354:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st627
	st627:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof627
		}
	st_case_627:
//line plugins/parsers/influx/machine.go:24416
		switch (m.data)[(m.p)] {
		case 10:
			goto tr755
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 65:
			goto st176
		case 92:
			goto st75
		case 97:
			goto st179
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st176:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof176
		}
	st_case_176:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 76:
			goto st177
		case 92:
			goto st75
		}
		goto st6
	st177:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof177
		}
	st_case_177:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 83:
			goto st178
		case 92:
			goto st75
		}
		goto st6
	st178:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof178
		}
	st_case_178:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 69:
			goto st628
		case 92:
			goto st75
		}
		goto st6
	st628:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof628
		}
	st_case_628:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr755
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st179:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof179
		}
	st_case_179:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 108:
			goto st180
		}
		goto st6
	st180:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof180
		}
	st_case_180:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 115:
			goto st181
		}
		goto st6
	st181:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof181
		}
	st_case_181:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 101:
			goto st628
		}
		goto st6
	tr355:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st629
	st629:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof629
		}
	st_case_629:
//line plugins/parsers/influx/machine.go:24597
		switch (m.data)[(m.p)] {
		case 10:
			goto tr755
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 82:
			goto st182
		case 92:
			goto st75
		case 114:
			goto st183
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st182:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof182
		}
	st_case_182:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 85:
			goto st178
		case 92:
			goto st75
		}
		goto st6
	st183:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof183
		}
	st_case_183:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 117:
			goto st181
		}
		goto st6
	tr356:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st630
	st630:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof630
		}
	st_case_630:
//line plugins/parsers/influx/machine.go:24673
		switch (m.data)[(m.p)] {
		case 10:
			goto tr755
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		case 97:
			goto st179
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	tr357:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st631
	st631:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof631
		}
	st_case_631:
//line plugins/parsers/influx/machine.go:24707
		switch (m.data)[(m.p)] {
		case 10:
			goto tr755
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		case 114:
			goto st183
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	tr349:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st184
	st184:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof184
		}
	st_case_184:
//line plugins/parsers/influx/machine.go:24741
		switch (m.data)[(m.p)] {
		case 34:
			goto st172
		case 92:
			goto st172
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	st632:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof632
		}
	st_case_632:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 46:
			goto st326
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		case 105:
			goto st624
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st620
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	st633:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof633
		}
	st_case_633:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr534
		case 12:
			goto tr535
		case 13:
			goto tr536
		case 32:
			goto tr533
		case 34:
			goto tr31
		case 44:
			goto tr537
		case 46:
			goto st326
		case 69:
			goto st174
		case 92:
			goto st75
		case 101:
			goto st174
		case 105:
			goto st624
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st633
			}
		case (m.data)[(m.p)] >= 9:
			goto tr533
		}
		goto st6
	tr171:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st634
	st634:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof634
		}
	st_case_634:
//line plugins/parsers/influx/machine.go:24844
		switch (m.data)[(m.p)] {
		case 10:
			goto tr900
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 65:
			goto st185
		case 92:
			goto st75
		case 97:
			goto st188
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st185:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof185
		}
	st_case_185:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 76:
			goto st186
		case 92:
			goto st75
		}
		goto st6
	st186:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof186
		}
	st_case_186:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 83:
			goto st187
		case 92:
			goto st75
		}
		goto st6
	st187:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof187
		}
	st_case_187:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 69:
			goto st635
		case 92:
			goto st75
		}
		goto st6
	st635:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof635
		}
	st_case_635:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr900
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st188:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof188
		}
	st_case_188:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 108:
			goto st189
		}
		goto st6
	st189:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof189
		}
	st_case_189:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 115:
			goto st190
		}
		goto st6
	st190:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof190
		}
	st_case_190:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 101:
			goto st635
		}
		goto st6
	tr172:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st636
	st636:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof636
		}
	st_case_636:
//line plugins/parsers/influx/machine.go:25025
		switch (m.data)[(m.p)] {
		case 10:
			goto tr900
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 82:
			goto st191
		case 92:
			goto st75
		case 114:
			goto st192
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	st191:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof191
		}
	st_case_191:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 85:
			goto st187
		case 92:
			goto st75
		}
		goto st6
	st192:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof192
		}
	st_case_192:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 117:
			goto st190
		}
		goto st6
	tr173:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st637
	st637:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof637
		}
	st_case_637:
//line plugins/parsers/influx/machine.go:25101
		switch (m.data)[(m.p)] {
		case 10:
			goto tr900
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		case 97:
			goto st188
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	tr174:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st638
	st638:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof638
		}
	st_case_638:
//line plugins/parsers/influx/machine.go:25135
		switch (m.data)[(m.p)] {
		case 10:
			goto tr900
		case 12:
			goto tr939
		case 13:
			goto tr758
		case 32:
			goto tr938
		case 34:
			goto tr31
		case 44:
			goto tr940
		case 92:
			goto st75
		case 114:
			goto st192
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr938
		}
		goto st6
	tr162:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st193
	st193:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof193
		}
	st_case_193:
//line plugins/parsers/influx/machine.go:25169
		switch (m.data)[(m.p)] {
		case 9:
			goto st49
		case 10:
			goto tr29
		case 11:
			goto tr162
		case 12:
			goto st2
		case 13:
			goto st7
		case 32:
			goto st49
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto tr165
		case 92:
			goto tr163
		}
		goto tr160
	tr140:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st194
	st194:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof194
		}
	st_case_194:
//line plugins/parsers/influx/machine.go:25204
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 46:
			goto st195
		case 48:
			goto st640
		case 61:
			goto tr47
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st643
			}
		case (m.data)[(m.p)] >= 9:
			goto tr60
		}
		goto st16
	tr141:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st195
	st195:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof195
		}
	st_case_195:
//line plugins/parsers/influx/machine.go:25245
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st639
			}
		case (m.data)[(m.p)] >= 9:
			goto tr60
		}
		goto st16
	st639:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof639
		}
	st_case_639:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st639
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	st196:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof196
		}
	st_case_196:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 34:
			goto st197
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] < 43:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
				goto tr60
			}
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st480
			}
		default:
			goto st197
		}
		goto st16
	st197:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof197
		}
	st_case_197:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st480
			}
		case (m.data)[(m.p)] >= 9:
			goto tr60
		}
		goto st16
	st640:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof640
		}
	st_case_640:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 46:
			goto st639
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		case 105:
			goto st642
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st641
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	st641:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof641
		}
	st_case_641:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 46:
			goto st639
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st641
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	st642:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof642
		}
	st_case_642:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr952
		case 11:
			goto tr953
		case 13:
			goto tr954
		case 32:
			goto tr743
		case 44:
			goto tr955
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr743
		}
		goto st16
	st643:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof643
		}
	st_case_643:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 46:
			goto st639
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		case 105:
			goto st642
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st643
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	tr142:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st644
	st644:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof644
		}
	st_case_644:
//line plugins/parsers/influx/machine.go:25519
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 46:
			goto st639
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		case 105:
			goto st642
		case 117:
			goto st645
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st641
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	st645:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof645
		}
	st_case_645:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr957
		case 11:
			goto tr958
		case 13:
			goto tr959
		case 32:
			goto tr750
		case 44:
			goto tr960
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr750
		}
		goto st16
	tr143:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st646
	st646:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof646
		}
	st_case_646:
//line plugins/parsers/influx/machine.go:25591
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr735
		case 13:
			goto tr736
		case 32:
			goto tr731
		case 44:
			goto tr737
		case 46:
			goto st639
		case 61:
			goto tr132
		case 69:
			goto st196
		case 92:
			goto st22
		case 101:
			goto st196
		case 105:
			goto st642
		case 117:
			goto st645
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st646
			}
		case (m.data)[(m.p)] >= 9:
			goto tr731
		}
		goto st16
	tr144:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st647
	st647:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof647
		}
	st_case_647:
//line plugins/parsers/influx/machine.go:25638
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr963
		case 13:
			goto tr964
		case 32:
			goto tr757
		case 44:
			goto tr965
		case 61:
			goto tr132
		case 65:
			goto st198
		case 92:
			goto st22
		case 97:
			goto st201
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr757
		}
		goto st16
	st198:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof198
		}
	st_case_198:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 76:
			goto st199
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st199:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof199
		}
	st_case_199:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 83:
			goto st200
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st200:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof200
		}
	st_case_200:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 69:
			goto st648
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st648:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof648
		}
	st_case_648:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr963
		case 13:
			goto tr964
		case 32:
			goto tr757
		case 44:
			goto tr965
		case 61:
			goto tr132
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr757
		}
		goto st16
	st201:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof201
		}
	st_case_201:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		case 108:
			goto st202
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st202:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof202
		}
	st_case_202:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		case 115:
			goto st203
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st203:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof203
		}
	st_case_203:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		case 101:
			goto st648
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	tr145:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st649
	st649:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof649
		}
	st_case_649:
//line plugins/parsers/influx/machine.go:25861
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr963
		case 13:
			goto tr964
		case 32:
			goto tr757
		case 44:
			goto tr965
		case 61:
			goto tr132
		case 82:
			goto st204
		case 92:
			goto st22
		case 114:
			goto st205
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr757
		}
		goto st16
	st204:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof204
		}
	st_case_204:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 85:
			goto st200
		case 92:
			goto st22
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	st205:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof205
		}
	st_case_205:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr61
		case 13:
			goto tr47
		case 32:
			goto tr60
		case 44:
			goto tr62
		case 61:
			goto tr47
		case 92:
			goto st22
		case 117:
			goto st203
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr60
		}
		goto st16
	tr146:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st650
	st650:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof650
		}
	st_case_650:
//line plugins/parsers/influx/machine.go:25951
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr963
		case 13:
			goto tr964
		case 32:
			goto tr757
		case 44:
			goto tr965
		case 61:
			goto tr132
		case 92:
			goto st22
		case 97:
			goto st201
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr757
		}
		goto st16
	tr147:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st651
	st651:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof651
		}
	st_case_651:
//line plugins/parsers/influx/machine.go:25985
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr963
		case 13:
			goto tr964
		case 32:
			goto tr757
		case 44:
			goto tr965
		case 61:
			goto tr132
		case 92:
			goto st22
		case 114:
			goto st205
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr757
		}
		goto st16
	tr123:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st206
	tr382:
		(m.cs) = 206
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st206:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof206
		}
	st_case_206:
//line plugins/parsers/influx/machine.go:26036
		switch (m.data)[(m.p)] {
		case 9:
			goto tr119
		case 10:
			goto tr29
		case 11:
			goto tr382
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr119
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 61:
			goto tr383
		case 92:
			goto tr125
		}
		goto tr121
	tr120:
		(m.cs) = 207
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st207:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof207
		}
	st_case_207:
//line plugins/parsers/influx/machine.go:26082
		switch (m.data)[(m.p)] {
		case 9:
			goto tr119
		case 10:
			goto tr29
		case 11:
			goto tr382
		case 12:
			goto tr38
		case 13:
			goto st7
		case 32:
			goto tr119
		case 34:
			goto tr124
		case 44:
			goto tr92
		case 61:
			goto tr82
		case 92:
			goto tr125
		}
		goto tr121
	tr499:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st208
	st208:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof208
		}
	st_case_208:
//line plugins/parsers/influx/machine.go:26117
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr105
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st652
		}
		goto st6
	tr500:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st652
	st652:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof652
		}
	st_case_652:
//line plugins/parsers/influx/machine.go:26145
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st653
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st653:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof653
		}
	st_case_653:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st654
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st654:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof654
		}
	st_case_654:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st655
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st655:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof655
		}
	st_case_655:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st656
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st656:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof656
		}
	st_case_656:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st657
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st657:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof657
		}
	st_case_657:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st658
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st658:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof658
		}
	st_case_658:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st659
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st659:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof659
		}
	st_case_659:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st660
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st660:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof660
		}
	st_case_660:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st661
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st661:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof661
		}
	st_case_661:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st662
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st662:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof662
		}
	st_case_662:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st663
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st663:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof663
		}
	st_case_663:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st664
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st664:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof664
		}
	st_case_664:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st665
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st665:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof665
		}
	st_case_665:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st666
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st666:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof666
		}
	st_case_666:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st667
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st667:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof667
		}
	st_case_667:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st668
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st668:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof668
		}
	st_case_668:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st669
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st669:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof669
		}
	st_case_669:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st670
			}
		case (m.data)[(m.p)] >= 9:
			goto tr602
		}
		goto st6
	st670:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof670
		}
	st_case_670:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr603
		case 12:
			goto tr469
		case 13:
			goto tr605
		case 32:
			goto tr602
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr602
		}
		goto st6
	tr496:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st209
	tr989:
		(m.cs) = 209
//line plugins/parsers/influx/machine.go.rl:122

		err = m.handler.AddFloat(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr994:
		(m.cs) = 209
//line plugins/parsers/influx/machine.go.rl:104

		err = m.handler.AddInt(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr997:
		(m.cs) = 209
//line plugins/parsers/influx/machine.go.rl:113

		err = m.handler.AddUint(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1000:
		(m.cs) = 209
//line plugins/parsers/influx/machine.go.rl:131

		err = m.handler.AddBool(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st209:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof209
		}
	st_case_209:
//line plugins/parsers/influx/machine.go:26731
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr386
		case 44:
			goto st6
		case 61:
			goto st6
		case 92:
			goto tr387
		}
		goto tr385
	tr385:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st210
	st210:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof210
		}
	st_case_210:
//line plugins/parsers/influx/machine.go:26764
		switch (m.data)[(m.p)] {
		case 9:
			goto st6
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 32:
			goto st6
		case 34:
			goto tr100
		case 44:
			goto st6
		case 61:
			goto tr389
		case 92:
			goto st224
		}
		goto st210
	tr389:
//line plugins/parsers/influx/machine.go.rl:100

		m.key = m.text()

		goto st211
	st211:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof211
		}
	st_case_211:
//line plugins/parsers/influx/machine.go:26797
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr353
		case 45:
			goto tr391
		case 46:
			goto tr392
		case 48:
			goto tr393
		case 70:
			goto tr112
		case 84:
			goto tr113
		case 92:
			goto st75
		case 102:
			goto tr114
		case 116:
			goto tr115
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto tr394
		}
		goto st6
	tr391:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st212
	st212:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof212
		}
	st_case_212:
//line plugins/parsers/influx/machine.go:26839
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 46:
			goto st213
		case 48:
			goto st673
		case 92:
			goto st75
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st676
		}
		goto st6
	tr392:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st213
	st213:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof213
		}
	st_case_213:
//line plugins/parsers/influx/machine.go:26871
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st671
		}
		goto st6
	st671:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof671
		}
	st_case_671:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st671
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st214:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof214
		}
	st_case_214:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr358
		case 43:
			goto st215
		case 45:
			goto st215
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st672
		}
		goto st6
	st215:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof215
		}
	st_case_215:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st672
		}
		goto st6
	st672:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof672
		}
	st_case_672:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st672
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st673:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof673
		}
	st_case_673:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st671
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		case 105:
			goto st675
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st674
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st674:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof674
		}
	st_case_674:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st671
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st674
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st675:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof675
		}
	st_case_675:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr798
		case 12:
			goto tr932
		case 13:
			goto tr800
		case 32:
			goto tr993
		case 34:
			goto tr31
		case 44:
			goto tr994
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr993
		}
		goto st6
	st676:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof676
		}
	st_case_676:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st671
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		case 105:
			goto st675
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st676
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	tr393:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st677
	st677:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof677
		}
	st_case_677:
//line plugins/parsers/influx/machine.go:27146
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st671
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		case 105:
			goto st675
		case 117:
			goto st678
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st674
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st678:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof678
		}
	st_case_678:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr804
		case 12:
			goto tr935
		case 13:
			goto tr806
		case 32:
			goto tr996
		case 34:
			goto tr31
		case 44:
			goto tr997
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr996
		}
		goto st6
	tr394:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st679
	st679:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof679
		}
	st_case_679:
//line plugins/parsers/influx/machine.go:27218
		switch (m.data)[(m.p)] {
		case 10:
			goto tr765
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st671
		case 69:
			goto st214
		case 92:
			goto st75
		case 101:
			goto st214
		case 105:
			goto st675
		case 117:
			goto st678
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st679
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	tr112:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st680
	st680:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof680
		}
	st_case_680:
//line plugins/parsers/influx/machine.go:27265
		switch (m.data)[(m.p)] {
		case 10:
			goto tr810
		case 12:
			goto tr939
		case 13:
			goto tr812
		case 32:
			goto tr999
		case 34:
			goto tr31
		case 44:
			goto tr1000
		case 65:
			goto st216
		case 92:
			goto st75
		case 97:
			goto st219
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr999
		}
		goto st6
	st216:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof216
		}
	st_case_216:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 76:
			goto st217
		case 92:
			goto st75
		}
		goto st6
	st217:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof217
		}
	st_case_217:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 83:
			goto st218
		case 92:
			goto st75
		}
		goto st6
	st218:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof218
		}
	st_case_218:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 69:
			goto st681
		case 92:
			goto st75
		}
		goto st6
	st681:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof681
		}
	st_case_681:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr810
		case 12:
			goto tr939
		case 13:
			goto tr812
		case 32:
			goto tr999
		case 34:
			goto tr31
		case 44:
			goto tr1000
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr999
		}
		goto st6
	st219:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof219
		}
	st_case_219:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 108:
			goto st220
		}
		goto st6
	st220:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof220
		}
	st_case_220:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 115:
			goto st221
		}
		goto st6
	st221:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof221
		}
	st_case_221:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 101:
			goto st681
		}
		goto st6
	tr113:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st682
	st682:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof682
		}
	st_case_682:
//line plugins/parsers/influx/machine.go:27446
		switch (m.data)[(m.p)] {
		case 10:
			goto tr810
		case 12:
			goto tr939
		case 13:
			goto tr812
		case 32:
			goto tr999
		case 34:
			goto tr31
		case 44:
			goto tr1000
		case 82:
			goto st222
		case 92:
			goto st75
		case 114:
			goto st223
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr999
		}
		goto st6
	st222:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof222
		}
	st_case_222:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 85:
			goto st218
		case 92:
			goto st75
		}
		goto st6
	st223:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof223
		}
	st_case_223:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		case 117:
			goto st221
		}
		goto st6
	tr114:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st683
	st683:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof683
		}
	st_case_683:
//line plugins/parsers/influx/machine.go:27522
		switch (m.data)[(m.p)] {
		case 10:
			goto tr810
		case 12:
			goto tr939
		case 13:
			goto tr812
		case 32:
			goto tr999
		case 34:
			goto tr31
		case 44:
			goto tr1000
		case 92:
			goto st75
		case 97:
			goto st219
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr999
		}
		goto st6
	tr115:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st684
	st684:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof684
		}
	st_case_684:
//line plugins/parsers/influx/machine.go:27556
		switch (m.data)[(m.p)] {
		case 10:
			goto tr810
		case 12:
			goto tr939
		case 13:
			goto tr812
		case 32:
			goto tr999
		case 34:
			goto tr31
		case 44:
			goto tr1000
		case 92:
			goto st75
		case 114:
			goto st223
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr999
		}
		goto st6
	tr387:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st224
	st224:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof224
		}
	st_case_224:
//line plugins/parsers/influx/machine.go:27590
		switch (m.data)[(m.p)] {
		case 34:
			goto st210
		case 92:
			goto st210
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr8
			}
		case (m.data)[(m.p)] >= 9:
			goto tr8
		}
		goto st3
	tr108:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st225
	st225:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof225
		}
	st_case_225:
//line plugins/parsers/influx/machine.go:27617
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 46:
			goto st226
		case 48:
			goto st687
		case 92:
			goto st75
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st690
		}
		goto st6
	tr109:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st226
	st226:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof226
		}
	st_case_226:
//line plugins/parsers/influx/machine.go:27649
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st685
		}
		goto st6
	st685:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof685
		}
	st_case_685:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st685
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st227:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof227
		}
	st_case_227:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr358
		case 43:
			goto st228
		case 45:
			goto st228
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st686
		}
		goto st6
	st228:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof228
		}
	st_case_228:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr29
		case 12:
			goto tr8
		case 13:
			goto st7
		case 34:
			goto tr31
		case 92:
			goto st75
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st686
		}
		goto st6
	st686:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof686
		}
	st_case_686:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 92:
			goto st75
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st686
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st687:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof687
		}
	st_case_687:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st685
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		case 105:
			goto st689
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st688
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st688:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof688
		}
	st_case_688:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st685
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st688
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st689:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof689
		}
	st_case_689:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr823
		case 12:
			goto tr932
		case 13:
			goto tr800
		case 32:
			goto tr993
		case 34:
			goto tr31
		case 44:
			goto tr994
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr993
		}
		goto st6
	st690:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof690
		}
	st_case_690:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st685
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		case 105:
			goto st689
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st690
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	tr110:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st691
	st691:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof691
		}
	st_case_691:
//line plugins/parsers/influx/machine.go:27924
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st685
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		case 105:
			goto st689
		case 117:
			goto st692
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st688
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	st692:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof692
		}
	st_case_692:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr829
		case 12:
			goto tr935
		case 13:
			goto tr806
		case 32:
			goto tr996
		case 34:
			goto tr31
		case 44:
			goto tr997
		case 92:
			goto st75
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 11 {
			goto tr996
		}
		goto st6
	tr111:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st693
	st693:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof693
		}
	st_case_693:
//line plugins/parsers/influx/machine.go:27996
		switch (m.data)[(m.p)] {
		case 10:
			goto tr639
		case 12:
			goto tr535
		case 13:
			goto tr642
		case 32:
			goto tr988
		case 34:
			goto tr31
		case 44:
			goto tr989
		case 46:
			goto st685
		case 69:
			goto st227
		case 92:
			goto st75
		case 101:
			goto st227
		case 105:
			goto st689
		case 117:
			goto st692
		}
		switch {
		case (m.data)[(m.p)] > 11:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st693
			}
		case (m.data)[(m.p)] >= 9:
			goto tr988
		}
		goto st6
	tr96:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st229
	st229:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof229
		}
	st_case_229:
//line plugins/parsers/influx/machine.go:28043
		switch (m.data)[(m.p)] {
		case 9:
			goto st31
		case 10:
			goto tr29
		case 11:
			goto tr96
		case 12:
			goto st2
		case 13:
			goto st7
		case 32:
			goto st31
		case 34:
			goto tr97
		case 44:
			goto st6
		case 61:
			goto tr101
		case 92:
			goto tr98
		}
		goto tr94
	tr74:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st230
	st230:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof230
		}
	st_case_230:
//line plugins/parsers/influx/machine.go:28078
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 46:
			goto st231
		case 48:
			goto st695
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st698
			}
		case (m.data)[(m.p)] >= 9:
			goto tr1
		}
		goto st1
	tr75:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st231
	st231:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof231
		}
	st_case_231:
//line plugins/parsers/influx/machine.go:28117
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st694
			}
		case (m.data)[(m.p)] >= 9:
			goto tr1
		}
		goto st1
	st694:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof694
		}
	st_case_694:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st694
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	st232:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof232
		}
	st_case_232:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 34:
			goto st233
		case 44:
			goto tr4
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] < 43:
			if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
				goto tr1
			}
		case (m.data)[(m.p)] > 45:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st531
			}
		default:
			goto st233
		}
		goto st1
	st233:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof233
		}
	st_case_233:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st531
			}
		case (m.data)[(m.p)] >= 9:
			goto tr1
		}
		goto st1
	st695:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof695
		}
	st_case_695:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 46:
			goto st694
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		case 105:
			goto st697
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st696
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	st696:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof696
		}
	st_case_696:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 46:
			goto st694
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st696
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	st697:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof697
		}
	st_case_697:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr952
		case 11:
			goto tr1013
		case 13:
			goto tr954
		case 32:
			goto tr825
		case 44:
			goto tr1014
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr825
		}
		goto st1
	st698:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof698
		}
	st_case_698:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 46:
			goto st694
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		case 105:
			goto st697
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st698
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	tr76:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st699
	st699:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof699
		}
	st_case_699:
//line plugins/parsers/influx/machine.go:28375
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 46:
			goto st694
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		case 105:
			goto st697
		case 117:
			goto st700
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st696
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	st700:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof700
		}
	st_case_700:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr957
		case 11:
			goto tr1016
		case 13:
			goto tr959
		case 32:
			goto tr831
		case 44:
			goto tr1017
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr831
		}
		goto st1
	tr77:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st701
	st701:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof701
		}
	st_case_701:
//line plugins/parsers/influx/machine.go:28443
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 11:
			goto tr818
		case 13:
			goto tr736
		case 32:
			goto tr641
		case 44:
			goto tr819
		case 46:
			goto st694
		case 69:
			goto st232
		case 92:
			goto st95
		case 101:
			goto st232
		case 105:
			goto st697
		case 117:
			goto st700
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st701
			}
		case (m.data)[(m.p)] >= 9:
			goto tr641
		}
		goto st1
	tr78:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st702
	st702:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof702
		}
	st_case_702:
//line plugins/parsers/influx/machine.go:28488
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr1019
		case 13:
			goto tr964
		case 32:
			goto tr836
		case 44:
			goto tr1020
		case 65:
			goto st234
		case 92:
			goto st95
		case 97:
			goto st237
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr836
		}
		goto st1
	st234:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof234
		}
	st_case_234:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 76:
			goto st235
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st235:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof235
		}
	st_case_235:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 83:
			goto st236
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st236:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof236
		}
	st_case_236:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 69:
			goto st703
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st703:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof703
		}
	st_case_703:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr1019
		case 13:
			goto tr964
		case 32:
			goto tr836
		case 44:
			goto tr1020
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr836
		}
		goto st1
	st237:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof237
		}
	st_case_237:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		case 108:
			goto st238
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st238:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof238
		}
	st_case_238:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		case 115:
			goto st239
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st239:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof239
		}
	st_case_239:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		case 101:
			goto st703
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	tr79:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st704
	st704:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof704
		}
	st_case_704:
//line plugins/parsers/influx/machine.go:28695
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr1019
		case 13:
			goto tr964
		case 32:
			goto tr836
		case 44:
			goto tr1020
		case 82:
			goto st240
		case 92:
			goto st95
		case 114:
			goto st241
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr836
		}
		goto st1
	st240:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof240
		}
	st_case_240:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 85:
			goto st236
		case 92:
			goto st95
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	st241:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof241
		}
	st_case_241:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr47
		case 11:
			goto tr3
		case 13:
			goto tr47
		case 32:
			goto tr1
		case 44:
			goto tr4
		case 92:
			goto st95
		case 117:
			goto st239
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr1
		}
		goto st1
	tr80:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st705
	st705:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof705
		}
	st_case_705:
//line plugins/parsers/influx/machine.go:28779
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr1019
		case 13:
			goto tr964
		case 32:
			goto tr836
		case 44:
			goto tr1020
		case 92:
			goto st95
		case 97:
			goto st237
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr836
		}
		goto st1
	tr81:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st706
	st706:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof706
		}
	st_case_706:
//line plugins/parsers/influx/machine.go:28811
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 11:
			goto tr1019
		case 13:
			goto tr964
		case 32:
			goto tr836
		case 44:
			goto tr1020
		case 92:
			goto st95
		case 114:
			goto st241
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr836
		}
		goto st1
	tr44:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st242
	tr424:
		(m.cs) = 242
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st242:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof242
		}
	st_case_242:
//line plugins/parsers/influx/machine.go:28860
		switch (m.data)[(m.p)] {
		case 10:
			goto tr423
		case 11:
			goto tr424
		case 13:
			goto tr423
		case 32:
			goto tr38
		case 44:
			goto tr4
		case 61:
			goto tr425
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr38
		}
		goto tr41
	tr40:
		(m.cs) = 243
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st243:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof243
		}
	st_case_243:
//line plugins/parsers/influx/machine.go:28903
		switch (m.data)[(m.p)] {
		case 10:
			goto tr423
		case 11:
			goto tr424
		case 13:
			goto tr423
		case 32:
			goto tr38
		case 44:
			goto tr4
		case 61:
			goto tr33
		case 92:
			goto tr45
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr38
		}
		goto tr41
	tr464:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st244
	st244:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof244
		}
	st_case_244:
//line plugins/parsers/influx/machine.go:28935
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st707
		}
		goto tr426
	tr465:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st707
	st707:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof707
		}
	st_case_707:
//line plugins/parsers/influx/machine.go:28951
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st708
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st708:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof708
		}
	st_case_708:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st709
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st709:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof709
		}
	st_case_709:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st710
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st710:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof710
		}
	st_case_710:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st711
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st711:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof711
		}
	st_case_711:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st712
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st712:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof712
		}
	st_case_712:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st713
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st713:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof713
		}
	st_case_713:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st714
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st714:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof714
		}
	st_case_714:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st715
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st715:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof715
		}
	st_case_715:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st716
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st716:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof716
		}
	st_case_716:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st717
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st717:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof717
		}
	st_case_717:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st718
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st718:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof718
		}
	st_case_718:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st719
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st719:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof719
		}
	st_case_719:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st720
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st720:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof720
		}
	st_case_720:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st721
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st721:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof721
		}
	st_case_721:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st722
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st722:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof722
		}
	st_case_722:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st723
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st723:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof723
		}
	st_case_723:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st724
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st724:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof724
		}
	st_case_724:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st725
			}
		case (m.data)[(m.p)] >= 9:
			goto tr469
		}
		goto tr426
	st725:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof725
		}
	st_case_725:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr470
		case 13:
			goto tr472
		case 32:
			goto tr469
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr469
		}
		goto tr426
	tr15:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st245
	st245:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof245
		}
	st_case_245:
//line plugins/parsers/influx/machine.go:29371
		switch (m.data)[(m.p)] {
		case 46:
			goto st246
		case 48:
			goto st727
		}
		if 49 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st730
		}
		goto tr8
	tr16:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st246
	st246:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof246
		}
	st_case_246:
//line plugins/parsers/influx/machine.go:29393
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st726
		}
		goto tr8
	st726:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof726
		}
	st_case_726:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 69:
			goto st247
		case 101:
			goto st247
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st726
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	st247:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof247
		}
	st_case_247:
		switch (m.data)[(m.p)] {
		case 34:
			goto st248
		case 43:
			goto st248
		case 45:
			goto st248
		}
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st622
		}
		goto tr8
	st248:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof248
		}
	st_case_248:
		if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
			goto st622
		}
		goto tr8
	st727:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof727
		}
	st_case_727:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 46:
			goto st726
		case 69:
			goto st247
		case 101:
			goto st247
		case 105:
			goto st729
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st728
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	st728:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof728
		}
	st_case_728:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 46:
			goto st726
		case 69:
			goto st247
		case 101:
			goto st247
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st728
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	st729:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof729
		}
	st_case_729:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr952
		case 13:
			goto tr954
		case 32:
			goto tr932
		case 44:
			goto tr1046
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr932
		}
		goto tr105
	st730:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof730
		}
	st_case_730:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 46:
			goto st726
		case 69:
			goto st247
		case 101:
			goto st247
		case 105:
			goto st729
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st730
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	tr17:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st731
	st731:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof731
		}
	st_case_731:
//line plugins/parsers/influx/machine.go:29576
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 46:
			goto st726
		case 69:
			goto st247
		case 101:
			goto st247
		case 105:
			goto st729
		case 117:
			goto st732
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st728
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	st732:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof732
		}
	st_case_732:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr957
		case 13:
			goto tr959
		case 32:
			goto tr935
		case 44:
			goto tr1048
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr935
		}
		goto tr105
	tr18:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st733
	st733:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof733
		}
	st_case_733:
//line plugins/parsers/influx/machine.go:29636
		switch (m.data)[(m.p)] {
		case 10:
			goto tr734
		case 13:
			goto tr736
		case 32:
			goto tr535
		case 44:
			goto tr930
		case 46:
			goto st726
		case 69:
			goto st247
		case 101:
			goto st247
		case 105:
			goto st729
		case 117:
			goto st732
		}
		switch {
		case (m.data)[(m.p)] > 12:
			if 48 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 57 {
				goto st733
			}
		case (m.data)[(m.p)] >= 9:
			goto tr535
		}
		goto tr105
	tr19:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st734
	st734:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof734
		}
	st_case_734:
//line plugins/parsers/influx/machine.go:29677
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 13:
			goto tr964
		case 32:
			goto tr939
		case 44:
			goto tr1050
		case 65:
			goto st249
		case 97:
			goto st252
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr939
		}
		goto tr105
	st249:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof249
		}
	st_case_249:
		if (m.data)[(m.p)] == 76 {
			goto st250
		}
		goto tr8
	st250:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof250
		}
	st_case_250:
		if (m.data)[(m.p)] == 83 {
			goto st251
		}
		goto tr8
	st251:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof251
		}
	st_case_251:
		if (m.data)[(m.p)] == 69 {
			goto st735
		}
		goto tr8
	st735:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof735
		}
	st_case_735:
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 13:
			goto tr964
		case 32:
			goto tr939
		case 44:
			goto tr1050
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr939
		}
		goto tr105
	st252:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof252
		}
	st_case_252:
		if (m.data)[(m.p)] == 108 {
			goto st253
		}
		goto tr8
	st253:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof253
		}
	st_case_253:
		if (m.data)[(m.p)] == 115 {
			goto st254
		}
		goto tr8
	st254:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof254
		}
	st_case_254:
		if (m.data)[(m.p)] == 101 {
			goto st735
		}
		goto tr8
	tr20:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st736
	st736:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof736
		}
	st_case_736:
//line plugins/parsers/influx/machine.go:29780
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 13:
			goto tr964
		case 32:
			goto tr939
		case 44:
			goto tr1050
		case 82:
			goto st255
		case 114:
			goto st256
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr939
		}
		goto tr105
	st255:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof255
		}
	st_case_255:
		if (m.data)[(m.p)] == 85 {
			goto st251
		}
		goto tr8
	st256:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof256
		}
	st_case_256:
		if (m.data)[(m.p)] == 117 {
			goto st254
		}
		goto tr8
	tr21:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st737
	st737:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof737
		}
	st_case_737:
//line plugins/parsers/influx/machine.go:29828
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 13:
			goto tr964
		case 32:
			goto tr939
		case 44:
			goto tr1050
		case 97:
			goto st252
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr939
		}
		goto tr105
	tr22:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st738
	st738:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof738
		}
	st_case_738:
//line plugins/parsers/influx/machine.go:29856
		switch (m.data)[(m.p)] {
		case 10:
			goto tr962
		case 13:
			goto tr964
		case 32:
			goto tr939
		case 44:
			goto tr1050
		case 114:
			goto st256
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto tr939
		}
		goto tr105
	tr9:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st257
	st257:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof257
		}
	st_case_257:
//line plugins/parsers/influx/machine.go:29884
		switch (m.data)[(m.p)] {
		case 10:
			goto tr8
		case 11:
			goto tr9
		case 13:
			goto tr8
		case 32:
			goto st2
		case 44:
			goto tr8
		case 61:
			goto tr12
		case 92:
			goto tr10
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st2
		}
		goto tr6
	st258:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof258
		}
	st_case_258:
		if (m.data)[(m.p)] == 10 {
			goto tr440
		}
		goto st258
	tr440:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

//line plugins/parsers/influx/machine.go.rl:70

		{
			goto st740
		}

		goto st739
	st739:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof739
		}
	st_case_739:
//line plugins/parsers/influx/machine.go:29931
		goto st0
	st261:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof261
		}
	st_case_261:
		switch (m.data)[(m.p)] {
		case 32:
			goto tr35
		case 35:
			goto tr35
		case 44:
			goto tr35
		case 92:
			goto tr444
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr35
			}
		case (m.data)[(m.p)] >= 9:
			goto tr35
		}
		goto tr443
	tr443:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st741
	st741:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof741
		}
	st_case_741:
//line plugins/parsers/influx/machine.go:29972
		switch (m.data)[(m.p)] {
		case 9:
			goto tr2
		case 10:
			goto tr1058
		case 12:
			goto tr2
		case 13:
			goto tr1059
		case 32:
			goto tr2
		case 44:
			goto tr1060
		case 92:
			goto st269
		}
		goto st741
	tr445:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st742
	tr1058:
		(m.cs) = 742
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	tr1062:
		(m.cs) = 742
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto _again
	st742:
//line plugins/parsers/influx/machine.go.rl:164

		m.finishMetric = true
		(m.cs) = 740
		{
			(m.p)++
			goto _out
		}

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof742
		}
	st_case_742:
//line plugins/parsers/influx/machine.go:30047
		goto st0
	tr1059:
		(m.cs) = 262
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1063:
		(m.cs) = 262
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st262:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof262
		}
	st_case_262:
//line plugins/parsers/influx/machine.go:30080
		if (m.data)[(m.p)] == 10 {
			goto tr445
		}
		goto st0
	tr1060:
		(m.cs) = 263
//line plugins/parsers/influx/machine.go.rl:78

		err = m.handler.SetMeasurement(m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	tr1064:
		(m.cs) = 263
//line plugins/parsers/influx/machine.go.rl:91

		err = m.handler.AddTag(m.key, m.text())
		if err != nil {
			(m.p)--

			(m.cs) = 258
			{
				(m.p)++
				goto _out
			}
		}

		goto _again
	st263:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof263
		}
	st_case_263:
//line plugins/parsers/influx/machine.go:30116
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr2
		case 92:
			goto tr447
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto tr446
	tr446:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st264
	st264:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof264
		}
	st_case_264:
//line plugins/parsers/influx/machine.go:30147
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr449
		case 92:
			goto st267
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st264
	tr449:
//line plugins/parsers/influx/machine.go.rl:87

		m.key = m.text()

		goto st265
	st265:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof265
		}
	st_case_265:
//line plugins/parsers/influx/machine.go:30178
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr2
		case 92:
			goto tr452
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto tr451
	tr451:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st743
	st743:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof743
		}
	st_case_743:
//line plugins/parsers/influx/machine.go:30209
		switch (m.data)[(m.p)] {
		case 9:
			goto tr2
		case 10:
			goto tr1062
		case 12:
			goto tr2
		case 13:
			goto tr1063
		case 32:
			goto tr2
		case 44:
			goto tr1064
		case 61:
			goto tr2
		case 92:
			goto st266
		}
		goto st743
	tr452:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st266
	st266:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof266
		}
	st_case_266:
//line plugins/parsers/influx/machine.go:30240
		if (m.data)[(m.p)] == 92 {
			goto st744
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st743
	st744:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof744
		}
	st_case_744:
//line plugins/parsers/influx/machine.go:30261
		switch (m.data)[(m.p)] {
		case 9:
			goto tr2
		case 10:
			goto tr1062
		case 12:
			goto tr2
		case 13:
			goto tr1063
		case 32:
			goto tr2
		case 44:
			goto tr1064
		case 61:
			goto tr2
		case 92:
			goto st266
		}
		goto st743
	tr447:
//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st267
	st267:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof267
		}
	st_case_267:
//line plugins/parsers/influx/machine.go:30292
		if (m.data)[(m.p)] == 92 {
			goto st268
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st264
	st268:
//line plugins/parsers/influx/machine.go.rl:240
		(m.p)--

		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof268
		}
	st_case_268:
//line plugins/parsers/influx/machine.go:30313
		switch (m.data)[(m.p)] {
		case 32:
			goto tr2
		case 44:
			goto tr2
		case 61:
			goto tr449
		case 92:
			goto st267
		}
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto tr2
			}
		case (m.data)[(m.p)] >= 9:
			goto tr2
		}
		goto st264
	tr444:
//line plugins/parsers/influx/machine.go.rl:74

		m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:20

		m.pb = m.p

		goto st269
	st269:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof269
		}
	st_case_269:
//line plugins/parsers/influx/machine.go:30348
		switch {
		case (m.data)[(m.p)] > 10:
			if 12 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 13 {
				goto st0
			}
		case (m.data)[(m.p)] >= 9:
			goto st0
		}
		goto st741
	tr441:
//line plugins/parsers/influx/machine.go.rl:158

		m.lineno++
		m.sol = m.p
		m.sol++ // next char will be the first column in the line

		goto st740
	st740:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof740
		}
	st_case_740:
//line plugins/parsers/influx/machine.go:30371
		switch (m.data)[(m.p)] {
		case 10:
			goto tr441
		case 13:
			goto st259
		case 32:
			goto st740
		case 35:
			goto st260
		}
		if 9 <= (m.data)[(m.p)] && (m.data)[(m.p)] <= 12 {
			goto st740
		}
		goto tr1055
	st259:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof259
		}
	st_case_259:
		if (m.data)[(m.p)] == 10 {
			goto tr441
		}
		goto st0
	st260:
		if (m.p)++; (m.p) == (m.pe) {
			goto _test_eof260
		}
	st_case_260:
		if (m.data)[(m.p)] == 10 {
			goto tr441
		}
		goto st260
	st_out:
	_test_eof270:
		(m.cs) = 270
		goto _test_eof
	_test_eof1:
		(m.cs) = 1
		goto _test_eof
	_test_eof2:
		(m.cs) = 2
		goto _test_eof
	_test_eof3:
		(m.cs) = 3
		goto _test_eof
	_test_eof4:
		(m.cs) = 4
		goto _test_eof
	_test_eof5:
		(m.cs) = 5
		goto _test_eof
	_test_eof6:
		(m.cs) = 6
		goto _test_eof
	_test_eof7:
		(m.cs) = 7
		goto _test_eof
	_test_eof271:
		(m.cs) = 271
		goto _test_eof
	_test_eof272:
		(m.cs) = 272
		goto _test_eof
	_test_eof273:
		(m.cs) = 273
		goto _test_eof
	_test_eof8:
		(m.cs) = 8
		goto _test_eof
	_test_eof9:
		(m.cs) = 9
		goto _test_eof
	_test_eof10:
		(m.cs) = 10
		goto _test_eof
	_test_eof11:
		(m.cs) = 11
		goto _test_eof
	_test_eof12:
		(m.cs) = 12
		goto _test_eof
	_test_eof13:
		(m.cs) = 13
		goto _test_eof
	_test_eof14:
		(m.cs) = 14
		goto _test_eof
	_test_eof15:
		(m.cs) = 15
		goto _test_eof
	_test_eof16:
		(m.cs) = 16
		goto _test_eof
	_test_eof17:
		(m.cs) = 17
		goto _test_eof
	_test_eof18:
		(m.cs) = 18
		goto _test_eof
	_test_eof19:
		(m.cs) = 19
		goto _test_eof
	_test_eof20:
		(m.cs) = 20
		goto _test_eof
	_test_eof21:
		(m.cs) = 21
		goto _test_eof
	_test_eof22:
		(m.cs) = 22
		goto _test_eof
	_test_eof23:
		(m.cs) = 23
		goto _test_eof
	_test_eof24:
		(m.cs) = 24
		goto _test_eof
	_test_eof25:
		(m.cs) = 25
		goto _test_eof
	_test_eof26:
		(m.cs) = 26
		goto _test_eof
	_test_eof27:
		(m.cs) = 27
		goto _test_eof
	_test_eof28:
		(m.cs) = 28
		goto _test_eof
	_test_eof29:
		(m.cs) = 29
		goto _test_eof
	_test_eof30:
		(m.cs) = 30
		goto _test_eof
	_test_eof31:
		(m.cs) = 31
		goto _test_eof
	_test_eof32:
		(m.cs) = 32
		goto _test_eof
	_test_eof274:
		(m.cs) = 274
		goto _test_eof
	_test_eof275:
		(m.cs) = 275
		goto _test_eof
	_test_eof33:
		(m.cs) = 33
		goto _test_eof
	_test_eof34:
		(m.cs) = 34
		goto _test_eof
	_test_eof276:
		(m.cs) = 276
		goto _test_eof
	_test_eof277:
		(m.cs) = 277
		goto _test_eof
	_test_eof278:
		(m.cs) = 278
		goto _test_eof
	_test_eof35:
		(m.cs) = 35
		goto _test_eof
	_test_eof279:
		(m.cs) = 279
		goto _test_eof
	_test_eof280:
		(m.cs) = 280
		goto _test_eof
	_test_eof281:
		(m.cs) = 281
		goto _test_eof
	_test_eof282:
		(m.cs) = 282
		goto _test_eof
	_test_eof283:
		(m.cs) = 283
		goto _test_eof
	_test_eof284:
		(m.cs) = 284
		goto _test_eof
	_test_eof285:
		(m.cs) = 285
		goto _test_eof
	_test_eof286:
		(m.cs) = 286
		goto _test_eof
	_test_eof287:
		(m.cs) = 287
		goto _test_eof
	_test_eof288:
		(m.cs) = 288
		goto _test_eof
	_test_eof289:
		(m.cs) = 289
		goto _test_eof
	_test_eof290:
		(m.cs) = 290
		goto _test_eof
	_test_eof291:
		(m.cs) = 291
		goto _test_eof
	_test_eof292:
		(m.cs) = 292
		goto _test_eof
	_test_eof293:
		(m.cs) = 293
		goto _test_eof
	_test_eof294:
		(m.cs) = 294
		goto _test_eof
	_test_eof295:
		(m.cs) = 295
		goto _test_eof
	_test_eof296:
		(m.cs) = 296
		goto _test_eof
	_test_eof36:
		(m.cs) = 36
		goto _test_eof
	_test_eof37:
		(m.cs) = 37
		goto _test_eof
	_test_eof297:
		(m.cs) = 297
		goto _test_eof
	_test_eof298:
		(m.cs) = 298
		goto _test_eof
	_test_eof299:
		(m.cs) = 299
		goto _test_eof
	_test_eof38:
		(m.cs) = 38
		goto _test_eof
	_test_eof39:
		(m.cs) = 39
		goto _test_eof
	_test_eof40:
		(m.cs) = 40
		goto _test_eof
	_test_eof41:
		(m.cs) = 41
		goto _test_eof
	_test_eof42:
		(m.cs) = 42
		goto _test_eof
	_test_eof300:
		(m.cs) = 300
		goto _test_eof
	_test_eof301:
		(m.cs) = 301
		goto _test_eof
	_test_eof302:
		(m.cs) = 302
		goto _test_eof
	_test_eof303:
		(m.cs) = 303
		goto _test_eof
	_test_eof43:
		(m.cs) = 43
		goto _test_eof
	_test_eof304:
		(m.cs) = 304
		goto _test_eof
	_test_eof305:
		(m.cs) = 305
		goto _test_eof
	_test_eof306:
		(m.cs) = 306
		goto _test_eof
	_test_eof307:
		(m.cs) = 307
		goto _test_eof
	_test_eof308:
		(m.cs) = 308
		goto _test_eof
	_test_eof309:
		(m.cs) = 309
		goto _test_eof
	_test_eof310:
		(m.cs) = 310
		goto _test_eof
	_test_eof311:
		(m.cs) = 311
		goto _test_eof
	_test_eof312:
		(m.cs) = 312
		goto _test_eof
	_test_eof313:
		(m.cs) = 313
		goto _test_eof
	_test_eof314:
		(m.cs) = 314
		goto _test_eof
	_test_eof315:
		(m.cs) = 315
		goto _test_eof
	_test_eof316:
		(m.cs) = 316
		goto _test_eof
	_test_eof317:
		(m.cs) = 317
		goto _test_eof
	_test_eof318:
		(m.cs) = 318
		goto _test_eof
	_test_eof319:
		(m.cs) = 319
		goto _test_eof
	_test_eof320:
		(m.cs) = 320
		goto _test_eof
	_test_eof321:
		(m.cs) = 321
		goto _test_eof
	_test_eof322:
		(m.cs) = 322
		goto _test_eof
	_test_eof323:
		(m.cs) = 323
		goto _test_eof
	_test_eof324:
		(m.cs) = 324
		goto _test_eof
	_test_eof325:
		(m.cs) = 325
		goto _test_eof
	_test_eof44:
		(m.cs) = 44
		goto _test_eof
	_test_eof45:
		(m.cs) = 45
		goto _test_eof
	_test_eof46:
		(m.cs) = 46
		goto _test_eof
	_test_eof47:
		(m.cs) = 47
		goto _test_eof
	_test_eof48:
		(m.cs) = 48
		goto _test_eof
	_test_eof49:
		(m.cs) = 49
		goto _test_eof
	_test_eof50:
		(m.cs) = 50
		goto _test_eof
	_test_eof51:
		(m.cs) = 51
		goto _test_eof
	_test_eof52:
		(m.cs) = 52
		goto _test_eof
	_test_eof53:
		(m.cs) = 53
		goto _test_eof
	_test_eof326:
		(m.cs) = 326
		goto _test_eof
	_test_eof327:
		(m.cs) = 327
		goto _test_eof
	_test_eof328:
		(m.cs) = 328
		goto _test_eof
	_test_eof54:
		(m.cs) = 54
		goto _test_eof
	_test_eof55:
		(m.cs) = 55
		goto _test_eof
	_test_eof56:
		(m.cs) = 56
		goto _test_eof
	_test_eof57:
		(m.cs) = 57
		goto _test_eof
	_test_eof58:
		(m.cs) = 58
		goto _test_eof
	_test_eof59:
		(m.cs) = 59
		goto _test_eof
	_test_eof329:
		(m.cs) = 329
		goto _test_eof
	_test_eof330:
		(m.cs) = 330
		goto _test_eof
	_test_eof60:
		(m.cs) = 60
		goto _test_eof
	_test_eof331:
		(m.cs) = 331
		goto _test_eof
	_test_eof332:
		(m.cs) = 332
		goto _test_eof
	_test_eof333:
		(m.cs) = 333
		goto _test_eof
	_test_eof334:
		(m.cs) = 334
		goto _test_eof
	_test_eof335:
		(m.cs) = 335
		goto _test_eof
	_test_eof336:
		(m.cs) = 336
		goto _test_eof
	_test_eof337:
		(m.cs) = 337
		goto _test_eof
	_test_eof338:
		(m.cs) = 338
		goto _test_eof
	_test_eof339:
		(m.cs) = 339
		goto _test_eof
	_test_eof340:
		(m.cs) = 340
		goto _test_eof
	_test_eof341:
		(m.cs) = 341
		goto _test_eof
	_test_eof342:
		(m.cs) = 342
		goto _test_eof
	_test_eof343:
		(m.cs) = 343
		goto _test_eof
	_test_eof344:
		(m.cs) = 344
		goto _test_eof
	_test_eof345:
		(m.cs) = 345
		goto _test_eof
	_test_eof346:
		(m.cs) = 346
		goto _test_eof
	_test_eof347:
		(m.cs) = 347
		goto _test_eof
	_test_eof348:
		(m.cs) = 348
		goto _test_eof
	_test_eof349:
		(m.cs) = 349
		goto _test_eof
	_test_eof350:
		(m.cs) = 350
		goto _test_eof
	_test_eof61:
		(m.cs) = 61
		goto _test_eof
	_test_eof351:
		(m.cs) = 351
		goto _test_eof
	_test_eof352:
		(m.cs) = 352
		goto _test_eof
	_test_eof353:
		(m.cs) = 353
		goto _test_eof
	_test_eof62:
		(m.cs) = 62
		goto _test_eof
	_test_eof354:
		(m.cs) = 354
		goto _test_eof
	_test_eof355:
		(m.cs) = 355
		goto _test_eof
	_test_eof356:
		(m.cs) = 356
		goto _test_eof
	_test_eof357:
		(m.cs) = 357
		goto _test_eof
	_test_eof358:
		(m.cs) = 358
		goto _test_eof
	_test_eof359:
		(m.cs) = 359
		goto _test_eof
	_test_eof360:
		(m.cs) = 360
		goto _test_eof
	_test_eof361:
		(m.cs) = 361
		goto _test_eof
	_test_eof362:
		(m.cs) = 362
		goto _test_eof
	_test_eof363:
		(m.cs) = 363
		goto _test_eof
	_test_eof364:
		(m.cs) = 364
		goto _test_eof
	_test_eof365:
		(m.cs) = 365
		goto _test_eof
	_test_eof366:
		(m.cs) = 366
		goto _test_eof
	_test_eof367:
		(m.cs) = 367
		goto _test_eof
	_test_eof368:
		(m.cs) = 368
		goto _test_eof
	_test_eof369:
		(m.cs) = 369
		goto _test_eof
	_test_eof370:
		(m.cs) = 370
		goto _test_eof
	_test_eof371:
		(m.cs) = 371
		goto _test_eof
	_test_eof372:
		(m.cs) = 372
		goto _test_eof
	_test_eof373:
		(m.cs) = 373
		goto _test_eof
	_test_eof63:
		(m.cs) = 63
		goto _test_eof
	_test_eof64:
		(m.cs) = 64
		goto _test_eof
	_test_eof65:
		(m.cs) = 65
		goto _test_eof
	_test_eof66:
		(m.cs) = 66
		goto _test_eof
	_test_eof67:
		(m.cs) = 67
		goto _test_eof
	_test_eof374:
		(m.cs) = 374
		goto _test_eof
	_test_eof68:
		(m.cs) = 68
		goto _test_eof
	_test_eof69:
		(m.cs) = 69
		goto _test_eof
	_test_eof70:
		(m.cs) = 70
		goto _test_eof
	_test_eof71:
		(m.cs) = 71
		goto _test_eof
	_test_eof72:
		(m.cs) = 72
		goto _test_eof
	_test_eof375:
		(m.cs) = 375
		goto _test_eof
	_test_eof376:
		(m.cs) = 376
		goto _test_eof
	_test_eof377:
		(m.cs) = 377
		goto _test_eof
	_test_eof73:
		(m.cs) = 73
		goto _test_eof
	_test_eof74:
		(m.cs) = 74
		goto _test_eof
	_test_eof378:
		(m.cs) = 378
		goto _test_eof
	_test_eof379:
		(m.cs) = 379
		goto _test_eof
	_test_eof75:
		(m.cs) = 75
		goto _test_eof
	_test_eof380:
		(m.cs) = 380
		goto _test_eof
	_test_eof76:
		(m.cs) = 76
		goto _test_eof
	_test_eof381:
		(m.cs) = 381
		goto _test_eof
	_test_eof382:
		(m.cs) = 382
		goto _test_eof
	_test_eof383:
		(m.cs) = 383
		goto _test_eof
	_test_eof384:
		(m.cs) = 384
		goto _test_eof
	_test_eof385:
		(m.cs) = 385
		goto _test_eof
	_test_eof386:
		(m.cs) = 386
		goto _test_eof
	_test_eof387:
		(m.cs) = 387
		goto _test_eof
	_test_eof388:
		(m.cs) = 388
		goto _test_eof
	_test_eof389:
		(m.cs) = 389
		goto _test_eof
	_test_eof390:
		(m.cs) = 390
		goto _test_eof
	_test_eof391:
		(m.cs) = 391
		goto _test_eof
	_test_eof392:
		(m.cs) = 392
		goto _test_eof
	_test_eof393:
		(m.cs) = 393
		goto _test_eof
	_test_eof394:
		(m.cs) = 394
		goto _test_eof
	_test_eof395:
		(m.cs) = 395
		goto _test_eof
	_test_eof396:
		(m.cs) = 396
		goto _test_eof
	_test_eof397:
		(m.cs) = 397
		goto _test_eof
	_test_eof398:
		(m.cs) = 398
		goto _test_eof
	_test_eof399:
		(m.cs) = 399
		goto _test_eof
	_test_eof400:
		(m.cs) = 400
		goto _test_eof
	_test_eof77:
		(m.cs) = 77
		goto _test_eof
	_test_eof78:
		(m.cs) = 78
		goto _test_eof
	_test_eof79:
		(m.cs) = 79
		goto _test_eof
	_test_eof80:
		(m.cs) = 80
		goto _test_eof
	_test_eof81:
		(m.cs) = 81
		goto _test_eof
	_test_eof82:
		(m.cs) = 82
		goto _test_eof
	_test_eof83:
		(m.cs) = 83
		goto _test_eof
	_test_eof84:
		(m.cs) = 84
		goto _test_eof
	_test_eof85:
		(m.cs) = 85
		goto _test_eof
	_test_eof86:
		(m.cs) = 86
		goto _test_eof
	_test_eof87:
		(m.cs) = 87
		goto _test_eof
	_test_eof88:
		(m.cs) = 88
		goto _test_eof
	_test_eof89:
		(m.cs) = 89
		goto _test_eof
	_test_eof90:
		(m.cs) = 90
		goto _test_eof
	_test_eof401:
		(m.cs) = 401
		goto _test_eof
	_test_eof402:
		(m.cs) = 402
		goto _test_eof
	_test_eof403:
		(m.cs) = 403
		goto _test_eof
	_test_eof404:
		(m.cs) = 404
		goto _test_eof
	_test_eof91:
		(m.cs) = 91
		goto _test_eof
	_test_eof92:
		(m.cs) = 92
		goto _test_eof
	_test_eof93:
		(m.cs) = 93
		goto _test_eof
	_test_eof94:
		(m.cs) = 94
		goto _test_eof
	_test_eof405:
		(m.cs) = 405
		goto _test_eof
	_test_eof406:
		(m.cs) = 406
		goto _test_eof
	_test_eof95:
		(m.cs) = 95
		goto _test_eof
	_test_eof96:
		(m.cs) = 96
		goto _test_eof
	_test_eof407:
		(m.cs) = 407
		goto _test_eof
	_test_eof97:
		(m.cs) = 97
		goto _test_eof
	_test_eof98:
		(m.cs) = 98
		goto _test_eof
	_test_eof408:
		(m.cs) = 408
		goto _test_eof
	_test_eof409:
		(m.cs) = 409
		goto _test_eof
	_test_eof99:
		(m.cs) = 99
		goto _test_eof
	_test_eof410:
		(m.cs) = 410
		goto _test_eof
	_test_eof411:
		(m.cs) = 411
		goto _test_eof
	_test_eof100:
		(m.cs) = 100
		goto _test_eof
	_test_eof101:
		(m.cs) = 101
		goto _test_eof
	_test_eof412:
		(m.cs) = 412
		goto _test_eof
	_test_eof413:
		(m.cs) = 413
		goto _test_eof
	_test_eof414:
		(m.cs) = 414
		goto _test_eof
	_test_eof415:
		(m.cs) = 415
		goto _test_eof
	_test_eof416:
		(m.cs) = 416
		goto _test_eof
	_test_eof417:
		(m.cs) = 417
		goto _test_eof
	_test_eof418:
		(m.cs) = 418
		goto _test_eof
	_test_eof419:
		(m.cs) = 419
		goto _test_eof
	_test_eof420:
		(m.cs) = 420
		goto _test_eof
	_test_eof421:
		(m.cs) = 421
		goto _test_eof
	_test_eof422:
		(m.cs) = 422
		goto _test_eof
	_test_eof423:
		(m.cs) = 423
		goto _test_eof
	_test_eof424:
		(m.cs) = 424
		goto _test_eof
	_test_eof425:
		(m.cs) = 425
		goto _test_eof
	_test_eof426:
		(m.cs) = 426
		goto _test_eof
	_test_eof427:
		(m.cs) = 427
		goto _test_eof
	_test_eof428:
		(m.cs) = 428
		goto _test_eof
	_test_eof429:
		(m.cs) = 429
		goto _test_eof
	_test_eof102:
		(m.cs) = 102
		goto _test_eof
	_test_eof430:
		(m.cs) = 430
		goto _test_eof
	_test_eof431:
		(m.cs) = 431
		goto _test_eof
	_test_eof432:
		(m.cs) = 432
		goto _test_eof
	_test_eof103:
		(m.cs) = 103
		goto _test_eof
	_test_eof104:
		(m.cs) = 104
		goto _test_eof
	_test_eof433:
		(m.cs) = 433
		goto _test_eof
	_test_eof434:
		(m.cs) = 434
		goto _test_eof
	_test_eof435:
		(m.cs) = 435
		goto _test_eof
	_test_eof105:
		(m.cs) = 105
		goto _test_eof
	_test_eof436:
		(m.cs) = 436
		goto _test_eof
	_test_eof437:
		(m.cs) = 437
		goto _test_eof
	_test_eof438:
		(m.cs) = 438
		goto _test_eof
	_test_eof439:
		(m.cs) = 439
		goto _test_eof
	_test_eof440:
		(m.cs) = 440
		goto _test_eof
	_test_eof441:
		(m.cs) = 441
		goto _test_eof
	_test_eof442:
		(m.cs) = 442
		goto _test_eof
	_test_eof443:
		(m.cs) = 443
		goto _test_eof
	_test_eof444:
		(m.cs) = 444
		goto _test_eof
	_test_eof445:
		(m.cs) = 445
		goto _test_eof
	_test_eof446:
		(m.cs) = 446
		goto _test_eof
	_test_eof447:
		(m.cs) = 447
		goto _test_eof
	_test_eof448:
		(m.cs) = 448
		goto _test_eof
	_test_eof449:
		(m.cs) = 449
		goto _test_eof
	_test_eof450:
		(m.cs) = 450
		goto _test_eof
	_test_eof451:
		(m.cs) = 451
		goto _test_eof
	_test_eof452:
		(m.cs) = 452
		goto _test_eof
	_test_eof453:
		(m.cs) = 453
		goto _test_eof
	_test_eof454:
		(m.cs) = 454
		goto _test_eof
	_test_eof455:
		(m.cs) = 455
		goto _test_eof
	_test_eof106:
		(m.cs) = 106
		goto _test_eof
	_test_eof456:
		(m.cs) = 456
		goto _test_eof
	_test_eof457:
		(m.cs) = 457
		goto _test_eof
	_test_eof458:
		(m.cs) = 458
		goto _test_eof
	_test_eof459:
		(m.cs) = 459
		goto _test_eof
	_test_eof460:
		(m.cs) = 460
		goto _test_eof
	_test_eof461:
		(m.cs) = 461
		goto _test_eof
	_test_eof462:
		(m.cs) = 462
		goto _test_eof
	_test_eof463:
		(m.cs) = 463
		goto _test_eof
	_test_eof464:
		(m.cs) = 464
		goto _test_eof
	_test_eof465:
		(m.cs) = 465
		goto _test_eof
	_test_eof466:
		(m.cs) = 466
		goto _test_eof
	_test_eof467:
		(m.cs) = 467
		goto _test_eof
	_test_eof468:
		(m.cs) = 468
		goto _test_eof
	_test_eof469:
		(m.cs) = 469
		goto _test_eof
	_test_eof470:
		(m.cs) = 470
		goto _test_eof
	_test_eof471:
		(m.cs) = 471
		goto _test_eof
	_test_eof472:
		(m.cs) = 472
		goto _test_eof
	_test_eof473:
		(m.cs) = 473
		goto _test_eof
	_test_eof474:
		(m.cs) = 474
		goto _test_eof
	_test_eof475:
		(m.cs) = 475
		goto _test_eof
	_test_eof476:
		(m.cs) = 476
		goto _test_eof
	_test_eof477:
		(m.cs) = 477
		goto _test_eof
	_test_eof107:
		(m.cs) = 107
		goto _test_eof
	_test_eof108:
		(m.cs) = 108
		goto _test_eof
	_test_eof109:
		(m.cs) = 109
		goto _test_eof
	_test_eof110:
		(m.cs) = 110
		goto _test_eof
	_test_eof111:
		(m.cs) = 111
		goto _test_eof
	_test_eof478:
		(m.cs) = 478
		goto _test_eof
	_test_eof112:
		(m.cs) = 112
		goto _test_eof
	_test_eof479:
		(m.cs) = 479
		goto _test_eof
	_test_eof480:
		(m.cs) = 480
		goto _test_eof
	_test_eof113:
		(m.cs) = 113
		goto _test_eof
	_test_eof481:
		(m.cs) = 481
		goto _test_eof
	_test_eof482:
		(m.cs) = 482
		goto _test_eof
	_test_eof483:
		(m.cs) = 483
		goto _test_eof
	_test_eof484:
		(m.cs) = 484
		goto _test_eof
	_test_eof485:
		(m.cs) = 485
		goto _test_eof
	_test_eof486:
		(m.cs) = 486
		goto _test_eof
	_test_eof487:
		(m.cs) = 487
		goto _test_eof
	_test_eof488:
		(m.cs) = 488
		goto _test_eof
	_test_eof489:
		(m.cs) = 489
		goto _test_eof
	_test_eof114:
		(m.cs) = 114
		goto _test_eof
	_test_eof115:
		(m.cs) = 115
		goto _test_eof
	_test_eof116:
		(m.cs) = 116
		goto _test_eof
	_test_eof490:
		(m.cs) = 490
		goto _test_eof
	_test_eof117:
		(m.cs) = 117
		goto _test_eof
	_test_eof118:
		(m.cs) = 118
		goto _test_eof
	_test_eof119:
		(m.cs) = 119
		goto _test_eof
	_test_eof491:
		(m.cs) = 491
		goto _test_eof
	_test_eof120:
		(m.cs) = 120
		goto _test_eof
	_test_eof121:
		(m.cs) = 121
		goto _test_eof
	_test_eof492:
		(m.cs) = 492
		goto _test_eof
	_test_eof493:
		(m.cs) = 493
		goto _test_eof
	_test_eof122:
		(m.cs) = 122
		goto _test_eof
	_test_eof123:
		(m.cs) = 123
		goto _test_eof
	_test_eof124:
		(m.cs) = 124
		goto _test_eof
	_test_eof125:
		(m.cs) = 125
		goto _test_eof
	_test_eof494:
		(m.cs) = 494
		goto _test_eof
	_test_eof495:
		(m.cs) = 495
		goto _test_eof
	_test_eof496:
		(m.cs) = 496
		goto _test_eof
	_test_eof126:
		(m.cs) = 126
		goto _test_eof
	_test_eof497:
		(m.cs) = 497
		goto _test_eof
	_test_eof498:
		(m.cs) = 498
		goto _test_eof
	_test_eof499:
		(m.cs) = 499
		goto _test_eof
	_test_eof500:
		(m.cs) = 500
		goto _test_eof
	_test_eof501:
		(m.cs) = 501
		goto _test_eof
	_test_eof502:
		(m.cs) = 502
		goto _test_eof
	_test_eof503:
		(m.cs) = 503
		goto _test_eof
	_test_eof504:
		(m.cs) = 504
		goto _test_eof
	_test_eof505:
		(m.cs) = 505
		goto _test_eof
	_test_eof506:
		(m.cs) = 506
		goto _test_eof
	_test_eof507:
		(m.cs) = 507
		goto _test_eof
	_test_eof508:
		(m.cs) = 508
		goto _test_eof
	_test_eof509:
		(m.cs) = 509
		goto _test_eof
	_test_eof510:
		(m.cs) = 510
		goto _test_eof
	_test_eof511:
		(m.cs) = 511
		goto _test_eof
	_test_eof512:
		(m.cs) = 512
		goto _test_eof
	_test_eof513:
		(m.cs) = 513
		goto _test_eof
	_test_eof514:
		(m.cs) = 514
		goto _test_eof
	_test_eof515:
		(m.cs) = 515
		goto _test_eof
	_test_eof516:
		(m.cs) = 516
		goto _test_eof
	_test_eof127:
		(m.cs) = 127
		goto _test_eof
	_test_eof128:
		(m.cs) = 128
		goto _test_eof
	_test_eof517:
		(m.cs) = 517
		goto _test_eof
	_test_eof518:
		(m.cs) = 518
		goto _test_eof
	_test_eof519:
		(m.cs) = 519
		goto _test_eof
	_test_eof520:
		(m.cs) = 520
		goto _test_eof
	_test_eof521:
		(m.cs) = 521
		goto _test_eof
	_test_eof522:
		(m.cs) = 522
		goto _test_eof
	_test_eof523:
		(m.cs) = 523
		goto _test_eof
	_test_eof524:
		(m.cs) = 524
		goto _test_eof
	_test_eof525:
		(m.cs) = 525
		goto _test_eof
	_test_eof129:
		(m.cs) = 129
		goto _test_eof
	_test_eof130:
		(m.cs) = 130
		goto _test_eof
	_test_eof131:
		(m.cs) = 131
		goto _test_eof
	_test_eof526:
		(m.cs) = 526
		goto _test_eof
	_test_eof132:
		(m.cs) = 132
		goto _test_eof
	_test_eof133:
		(m.cs) = 133
		goto _test_eof
	_test_eof134:
		(m.cs) = 134
		goto _test_eof
	_test_eof527:
		(m.cs) = 527
		goto _test_eof
	_test_eof135:
		(m.cs) = 135
		goto _test_eof
	_test_eof136:
		(m.cs) = 136
		goto _test_eof
	_test_eof528:
		(m.cs) = 528
		goto _test_eof
	_test_eof529:
		(m.cs) = 529
		goto _test_eof
	_test_eof137:
		(m.cs) = 137
		goto _test_eof
	_test_eof138:
		(m.cs) = 138
		goto _test_eof
	_test_eof139:
		(m.cs) = 139
		goto _test_eof
	_test_eof530:
		(m.cs) = 530
		goto _test_eof
	_test_eof531:
		(m.cs) = 531
		goto _test_eof
	_test_eof140:
		(m.cs) = 140
		goto _test_eof
	_test_eof532:
		(m.cs) = 532
		goto _test_eof
	_test_eof141:
		(m.cs) = 141
		goto _test_eof
	_test_eof533:
		(m.cs) = 533
		goto _test_eof
	_test_eof534:
		(m.cs) = 534
		goto _test_eof
	_test_eof535:
		(m.cs) = 535
		goto _test_eof
	_test_eof536:
		(m.cs) = 536
		goto _test_eof
	_test_eof537:
		(m.cs) = 537
		goto _test_eof
	_test_eof538:
		(m.cs) = 538
		goto _test_eof
	_test_eof539:
		(m.cs) = 539
		goto _test_eof
	_test_eof540:
		(m.cs) = 540
		goto _test_eof
	_test_eof142:
		(m.cs) = 142
		goto _test_eof
	_test_eof143:
		(m.cs) = 143
		goto _test_eof
	_test_eof144:
		(m.cs) = 144
		goto _test_eof
	_test_eof541:
		(m.cs) = 541
		goto _test_eof
	_test_eof145:
		(m.cs) = 145
		goto _test_eof
	_test_eof146:
		(m.cs) = 146
		goto _test_eof
	_test_eof147:
		(m.cs) = 147
		goto _test_eof
	_test_eof542:
		(m.cs) = 542
		goto _test_eof
	_test_eof148:
		(m.cs) = 148
		goto _test_eof
	_test_eof149:
		(m.cs) = 149
		goto _test_eof
	_test_eof543:
		(m.cs) = 543
		goto _test_eof
	_test_eof544:
		(m.cs) = 544
		goto _test_eof
	_test_eof545:
		(m.cs) = 545
		goto _test_eof
	_test_eof546:
		(m.cs) = 546
		goto _test_eof
	_test_eof547:
		(m.cs) = 547
		goto _test_eof
	_test_eof548:
		(m.cs) = 548
		goto _test_eof
	_test_eof549:
		(m.cs) = 549
		goto _test_eof
	_test_eof550:
		(m.cs) = 550
		goto _test_eof
	_test_eof551:
		(m.cs) = 551
		goto _test_eof
	_test_eof552:
		(m.cs) = 552
		goto _test_eof
	_test_eof553:
		(m.cs) = 553
		goto _test_eof
	_test_eof554:
		(m.cs) = 554
		goto _test_eof
	_test_eof555:
		(m.cs) = 555
		goto _test_eof
	_test_eof556:
		(m.cs) = 556
		goto _test_eof
	_test_eof557:
		(m.cs) = 557
		goto _test_eof
	_test_eof558:
		(m.cs) = 558
		goto _test_eof
	_test_eof559:
		(m.cs) = 559
		goto _test_eof
	_test_eof560:
		(m.cs) = 560
		goto _test_eof
	_test_eof561:
		(m.cs) = 561
		goto _test_eof
	_test_eof562:
		(m.cs) = 562
		goto _test_eof
	_test_eof150:
		(m.cs) = 150
		goto _test_eof
	_test_eof151:
		(m.cs) = 151
		goto _test_eof
	_test_eof563:
		(m.cs) = 563
		goto _test_eof
	_test_eof564:
		(m.cs) = 564
		goto _test_eof
	_test_eof565:
		(m.cs) = 565
		goto _test_eof
	_test_eof152:
		(m.cs) = 152
		goto _test_eof
	_test_eof566:
		(m.cs) = 566
		goto _test_eof
	_test_eof567:
		(m.cs) = 567
		goto _test_eof
	_test_eof153:
		(m.cs) = 153
		goto _test_eof
	_test_eof568:
		(m.cs) = 568
		goto _test_eof
	_test_eof569:
		(m.cs) = 569
		goto _test_eof
	_test_eof570:
		(m.cs) = 570
		goto _test_eof
	_test_eof571:
		(m.cs) = 571
		goto _test_eof
	_test_eof572:
		(m.cs) = 572
		goto _test_eof
	_test_eof573:
		(m.cs) = 573
		goto _test_eof
	_test_eof574:
		(m.cs) = 574
		goto _test_eof
	_test_eof575:
		(m.cs) = 575
		goto _test_eof
	_test_eof576:
		(m.cs) = 576
		goto _test_eof
	_test_eof577:
		(m.cs) = 577
		goto _test_eof
	_test_eof578:
		(m.cs) = 578
		goto _test_eof
	_test_eof579:
		(m.cs) = 579
		goto _test_eof
	_test_eof580:
		(m.cs) = 580
		goto _test_eof
	_test_eof581:
		(m.cs) = 581
		goto _test_eof
	_test_eof582:
		(m.cs) = 582
		goto _test_eof
	_test_eof583:
		(m.cs) = 583
		goto _test_eof
	_test_eof584:
		(m.cs) = 584
		goto _test_eof
	_test_eof585:
		(m.cs) = 585
		goto _test_eof
	_test_eof154:
		(m.cs) = 154
		goto _test_eof
	_test_eof155:
		(m.cs) = 155
		goto _test_eof
	_test_eof586:
		(m.cs) = 586
		goto _test_eof
	_test_eof156:
		(m.cs) = 156
		goto _test_eof
	_test_eof587:
		(m.cs) = 587
		goto _test_eof
	_test_eof588:
		(m.cs) = 588
		goto _test_eof
	_test_eof589:
		(m.cs) = 589
		goto _test_eof
	_test_eof590:
		(m.cs) = 590
		goto _test_eof
	_test_eof591:
		(m.cs) = 591
		goto _test_eof
	_test_eof592:
		(m.cs) = 592
		goto _test_eof
	_test_eof593:
		(m.cs) = 593
		goto _test_eof
	_test_eof594:
		(m.cs) = 594
		goto _test_eof
	_test_eof157:
		(m.cs) = 157
		goto _test_eof
	_test_eof158:
		(m.cs) = 158
		goto _test_eof
	_test_eof159:
		(m.cs) = 159
		goto _test_eof
	_test_eof595:
		(m.cs) = 595
		goto _test_eof
	_test_eof160:
		(m.cs) = 160
		goto _test_eof
	_test_eof161:
		(m.cs) = 161
		goto _test_eof
	_test_eof162:
		(m.cs) = 162
		goto _test_eof
	_test_eof596:
		(m.cs) = 596
		goto _test_eof
	_test_eof163:
		(m.cs) = 163
		goto _test_eof
	_test_eof164:
		(m.cs) = 164
		goto _test_eof
	_test_eof597:
		(m.cs) = 597
		goto _test_eof
	_test_eof598:
		(m.cs) = 598
		goto _test_eof
	_test_eof165:
		(m.cs) = 165
		goto _test_eof
	_test_eof166:
		(m.cs) = 166
		goto _test_eof
	_test_eof167:
		(m.cs) = 167
		goto _test_eof
	_test_eof168:
		(m.cs) = 168
		goto _test_eof
	_test_eof169:
		(m.cs) = 169
		goto _test_eof
	_test_eof170:
		(m.cs) = 170
		goto _test_eof
	_test_eof599:
		(m.cs) = 599
		goto _test_eof
	_test_eof600:
		(m.cs) = 600
		goto _test_eof
	_test_eof601:
		(m.cs) = 601
		goto _test_eof
	_test_eof602:
		(m.cs) = 602
		goto _test_eof
	_test_eof603:
		(m.cs) = 603
		goto _test_eof
	_test_eof604:
		(m.cs) = 604
		goto _test_eof
	_test_eof605:
		(m.cs) = 605
		goto _test_eof
	_test_eof606:
		(m.cs) = 606
		goto _test_eof
	_test_eof607:
		(m.cs) = 607
		goto _test_eof
	_test_eof608:
		(m.cs) = 608
		goto _test_eof
	_test_eof609:
		(m.cs) = 609
		goto _test_eof
	_test_eof610:
		(m.cs) = 610
		goto _test_eof
	_test_eof611:
		(m.cs) = 611
		goto _test_eof
	_test_eof612:
		(m.cs) = 612
		goto _test_eof
	_test_eof613:
		(m.cs) = 613
		goto _test_eof
	_test_eof614:
		(m.cs) = 614
		goto _test_eof
	_test_eof615:
		(m.cs) = 615
		goto _test_eof
	_test_eof616:
		(m.cs) = 616
		goto _test_eof
	_test_eof617:
		(m.cs) = 617
		goto _test_eof
	_test_eof171:
		(m.cs) = 171
		goto _test_eof
	_test_eof172:
		(m.cs) = 172
		goto _test_eof
	_test_eof173:
		(m.cs) = 173
		goto _test_eof
	_test_eof618:
		(m.cs) = 618
		goto _test_eof
	_test_eof619:
		(m.cs) = 619
		goto _test_eof
	_test_eof620:
		(m.cs) = 620
		goto _test_eof
	_test_eof174:
		(m.cs) = 174
		goto _test_eof
	_test_eof621:
		(m.cs) = 621
		goto _test_eof
	_test_eof622:
		(m.cs) = 622
		goto _test_eof
	_test_eof175:
		(m.cs) = 175
		goto _test_eof
	_test_eof623:
		(m.cs) = 623
		goto _test_eof
	_test_eof624:
		(m.cs) = 624
		goto _test_eof
	_test_eof625:
		(m.cs) = 625
		goto _test_eof
	_test_eof626:
		(m.cs) = 626
		goto _test_eof
	_test_eof627:
		(m.cs) = 627
		goto _test_eof
	_test_eof176:
		(m.cs) = 176
		goto _test_eof
	_test_eof177:
		(m.cs) = 177
		goto _test_eof
	_test_eof178:
		(m.cs) = 178
		goto _test_eof
	_test_eof628:
		(m.cs) = 628
		goto _test_eof
	_test_eof179:
		(m.cs) = 179
		goto _test_eof
	_test_eof180:
		(m.cs) = 180
		goto _test_eof
	_test_eof181:
		(m.cs) = 181
		goto _test_eof
	_test_eof629:
		(m.cs) = 629
		goto _test_eof
	_test_eof182:
		(m.cs) = 182
		goto _test_eof
	_test_eof183:
		(m.cs) = 183
		goto _test_eof
	_test_eof630:
		(m.cs) = 630
		goto _test_eof
	_test_eof631:
		(m.cs) = 631
		goto _test_eof
	_test_eof184:
		(m.cs) = 184
		goto _test_eof
	_test_eof632:
		(m.cs) = 632
		goto _test_eof
	_test_eof633:
		(m.cs) = 633
		goto _test_eof
	_test_eof634:
		(m.cs) = 634
		goto _test_eof
	_test_eof185:
		(m.cs) = 185
		goto _test_eof
	_test_eof186:
		(m.cs) = 186
		goto _test_eof
	_test_eof187:
		(m.cs) = 187
		goto _test_eof
	_test_eof635:
		(m.cs) = 635
		goto _test_eof
	_test_eof188:
		(m.cs) = 188
		goto _test_eof
	_test_eof189:
		(m.cs) = 189
		goto _test_eof
	_test_eof190:
		(m.cs) = 190
		goto _test_eof
	_test_eof636:
		(m.cs) = 636
		goto _test_eof
	_test_eof191:
		(m.cs) = 191
		goto _test_eof
	_test_eof192:
		(m.cs) = 192
		goto _test_eof
	_test_eof637:
		(m.cs) = 637
		goto _test_eof
	_test_eof638:
		(m.cs) = 638
		goto _test_eof
	_test_eof193:
		(m.cs) = 193
		goto _test_eof
	_test_eof194:
		(m.cs) = 194
		goto _test_eof
	_test_eof195:
		(m.cs) = 195
		goto _test_eof
	_test_eof639:
		(m.cs) = 639
		goto _test_eof
	_test_eof196:
		(m.cs) = 196
		goto _test_eof
	_test_eof197:
		(m.cs) = 197
		goto _test_eof
	_test_eof640:
		(m.cs) = 640
		goto _test_eof
	_test_eof641:
		(m.cs) = 641
		goto _test_eof
	_test_eof642:
		(m.cs) = 642
		goto _test_eof
	_test_eof643:
		(m.cs) = 643
		goto _test_eof
	_test_eof644:
		(m.cs) = 644
		goto _test_eof
	_test_eof645:
		(m.cs) = 645
		goto _test_eof
	_test_eof646:
		(m.cs) = 646
		goto _test_eof
	_test_eof647:
		(m.cs) = 647
		goto _test_eof
	_test_eof198:
		(m.cs) = 198
		goto _test_eof
	_test_eof199:
		(m.cs) = 199
		goto _test_eof
	_test_eof200:
		(m.cs) = 200
		goto _test_eof
	_test_eof648:
		(m.cs) = 648
		goto _test_eof
	_test_eof201:
		(m.cs) = 201
		goto _test_eof
	_test_eof202:
		(m.cs) = 202
		goto _test_eof
	_test_eof203:
		(m.cs) = 203
		goto _test_eof
	_test_eof649:
		(m.cs) = 649
		goto _test_eof
	_test_eof204:
		(m.cs) = 204
		goto _test_eof
	_test_eof205:
		(m.cs) = 205
		goto _test_eof
	_test_eof650:
		(m.cs) = 650
		goto _test_eof
	_test_eof651:
		(m.cs) = 651
		goto _test_eof
	_test_eof206:
		(m.cs) = 206
		goto _test_eof
	_test_eof207:
		(m.cs) = 207
		goto _test_eof
	_test_eof208:
		(m.cs) = 208
		goto _test_eof
	_test_eof652:
		(m.cs) = 652
		goto _test_eof
	_test_eof653:
		(m.cs) = 653
		goto _test_eof
	_test_eof654:
		(m.cs) = 654
		goto _test_eof
	_test_eof655:
		(m.cs) = 655
		goto _test_eof
	_test_eof656:
		(m.cs) = 656
		goto _test_eof
	_test_eof657:
		(m.cs) = 657
		goto _test_eof
	_test_eof658:
		(m.cs) = 658
		goto _test_eof
	_test_eof659:
		(m.cs) = 659
		goto _test_eof
	_test_eof660:
		(m.cs) = 660
		goto _test_eof
	_test_eof661:
		(m.cs) = 661
		goto _test_eof
	_test_eof662:
		(m.cs) = 662
		goto _test_eof
	_test_eof663:
		(m.cs) = 663
		goto _test_eof
	_test_eof664:
		(m.cs) = 664
		goto _test_eof
	_test_eof665:
		(m.cs) = 665
		goto _test_eof
	_test_eof666:
		(m.cs) = 666
		goto _test_eof
	_test_eof667:
		(m.cs) = 667
		goto _test_eof
	_test_eof668:
		(m.cs) = 668
		goto _test_eof
	_test_eof669:
		(m.cs) = 669
		goto _test_eof
	_test_eof670:
		(m.cs) = 670
		goto _test_eof
	_test_eof209:
		(m.cs) = 209
		goto _test_eof
	_test_eof210:
		(m.cs) = 210
		goto _test_eof
	_test_eof211:
		(m.cs) = 211
		goto _test_eof
	_test_eof212:
		(m.cs) = 212
		goto _test_eof
	_test_eof213:
		(m.cs) = 213
		goto _test_eof
	_test_eof671:
		(m.cs) = 671
		goto _test_eof
	_test_eof214:
		(m.cs) = 214
		goto _test_eof
	_test_eof215:
		(m.cs) = 215
		goto _test_eof
	_test_eof672:
		(m.cs) = 672
		goto _test_eof
	_test_eof673:
		(m.cs) = 673
		goto _test_eof
	_test_eof674:
		(m.cs) = 674
		goto _test_eof
	_test_eof675:
		(m.cs) = 675
		goto _test_eof
	_test_eof676:
		(m.cs) = 676
		goto _test_eof
	_test_eof677:
		(m.cs) = 677
		goto _test_eof
	_test_eof678:
		(m.cs) = 678
		goto _test_eof
	_test_eof679:
		(m.cs) = 679
		goto _test_eof
	_test_eof680:
		(m.cs) = 680
		goto _test_eof
	_test_eof216:
		(m.cs) = 216
		goto _test_eof
	_test_eof217:
		(m.cs) = 217
		goto _test_eof
	_test_eof218:
		(m.cs) = 218
		goto _test_eof
	_test_eof681:
		(m.cs) = 681
		goto _test_eof
	_test_eof219:
		(m.cs) = 219
		goto _test_eof
	_test_eof220:
		(m.cs) = 220
		goto _test_eof
	_test_eof221:
		(m.cs) = 221
		goto _test_eof
	_test_eof682:
		(m.cs) = 682
		goto _test_eof
	_test_eof222:
		(m.cs) = 222
		goto _test_eof
	_test_eof223:
		(m.cs) = 223
		goto _test_eof
	_test_eof683:
		(m.cs) = 683
		goto _test_eof
	_test_eof684:
		(m.cs) = 684
		goto _test_eof
	_test_eof224:
		(m.cs) = 224
		goto _test_eof
	_test_eof225:
		(m.cs) = 225
		goto _test_eof
	_test_eof226:
		(m.cs) = 226
		goto _test_eof
	_test_eof685:
		(m.cs) = 685
		goto _test_eof
	_test_eof227:
		(m.cs) = 227
		goto _test_eof
	_test_eof228:
		(m.cs) = 228
		goto _test_eof
	_test_eof686:
		(m.cs) = 686
		goto _test_eof
	_test_eof687:
		(m.cs) = 687
		goto _test_eof
	_test_eof688:
		(m.cs) = 688
		goto _test_eof
	_test_eof689:
		(m.cs) = 689
		goto _test_eof
	_test_eof690:
		(m.cs) = 690
		goto _test_eof
	_test_eof691:
		(m.cs) = 691
		goto _test_eof
	_test_eof692:
		(m.cs) = 692
		goto _test_eof
	_test_eof693:
		(m.cs) = 693
		goto _test_eof
	_test_eof229:
		(m.cs) = 229
		goto _test_eof
	_test_eof230:
		(m.cs) = 230
		goto _test_eof
	_test_eof231:
		(m.cs) = 231
		goto _test_eof
	_test_eof694:
		(m.cs) = 694
		goto _test_eof
	_test_eof232:
		(m.cs) = 232
		goto _test_eof
	_test_eof233:
		(m.cs) = 233
		goto _test_eof
	_test_eof695:
		(m.cs) = 695
		goto _test_eof
	_test_eof696:
		(m.cs) = 696
		goto _test_eof
	_test_eof697:
		(m.cs) = 697
		goto _test_eof
	_test_eof698:
		(m.cs) = 698
		goto _test_eof
	_test_eof699:
		(m.cs) = 699
		goto _test_eof
	_test_eof700:
		(m.cs) = 700
		goto _test_eof
	_test_eof701:
		(m.cs) = 701
		goto _test_eof
	_test_eof702:
		(m.cs) = 702
		goto _test_eof
	_test_eof234:
		(m.cs) = 234
		goto _test_eof
	_test_eof235:
		(m.cs) = 235
		goto _test_eof
	_test_eof236:
		(m.cs) = 236
		goto _test_eof
	_test_eof703:
		(m.cs) = 703
		goto _test_eof
	_test_eof237:
		(m.cs) = 237
		goto _test_eof
	_test_eof238:
		(m.cs) = 238
		goto _test_eof
	_test_eof239:
		(m.cs) = 239
		goto _test_eof
	_test_eof704:
		(m.cs) = 704
		goto _test_eof
	_test_eof240:
		(m.cs) = 240
		goto _test_eof
	_test_eof241:
		(m.cs) = 241
		goto _test_eof
	_test_eof705:
		(m.cs) = 705
		goto _test_eof
	_test_eof706:
		(m.cs) = 706
		goto _test_eof
	_test_eof242:
		(m.cs) = 242
		goto _test_eof
	_test_eof243:
		(m.cs) = 243
		goto _test_eof
	_test_eof244:
		(m.cs) = 244
		goto _test_eof
	_test_eof707:
		(m.cs) = 707
		goto _test_eof
	_test_eof708:
		(m.cs) = 708
		goto _test_eof
	_test_eof709:
		(m.cs) = 709
		goto _test_eof
	_test_eof710:
		(m.cs) = 710
		goto _test_eof
	_test_eof711:
		(m.cs) = 711
		goto _test_eof
	_test_eof712:
		(m.cs) = 712
		goto _test_eof
	_test_eof713:
		(m.cs) = 713
		goto _test_eof
	_test_eof714:
		(m.cs) = 714
		goto _test_eof
	_test_eof715:
		(m.cs) = 715
		goto _test_eof
	_test_eof716:
		(m.cs) = 716
		goto _test_eof
	_test_eof717:
		(m.cs) = 717
		goto _test_eof
	_test_eof718:
		(m.cs) = 718
		goto _test_eof
	_test_eof719:
		(m.cs) = 719
		goto _test_eof
	_test_eof720:
		(m.cs) = 720
		goto _test_eof
	_test_eof721:
		(m.cs) = 721
		goto _test_eof
	_test_eof722:
		(m.cs) = 722
		goto _test_eof
	_test_eof723:
		(m.cs) = 723
		goto _test_eof
	_test_eof724:
		(m.cs) = 724
		goto _test_eof
	_test_eof725:
		(m.cs) = 725
		goto _test_eof
	_test_eof245:
		(m.cs) = 245
		goto _test_eof
	_test_eof246:
		(m.cs) = 246
		goto _test_eof
	_test_eof726:
		(m.cs) = 726
		goto _test_eof
	_test_eof247:
		(m.cs) = 247
		goto _test_eof
	_test_eof248:
		(m.cs) = 248
		goto _test_eof
	_test_eof727:
		(m.cs) = 727
		goto _test_eof
	_test_eof728:
		(m.cs) = 728
		goto _test_eof
	_test_eof729:
		(m.cs) = 729
		goto _test_eof
	_test_eof730:
		(m.cs) = 730
		goto _test_eof
	_test_eof731:
		(m.cs) = 731
		goto _test_eof
	_test_eof732:
		(m.cs) = 732
		goto _test_eof
	_test_eof733:
		(m.cs) = 733
		goto _test_eof
	_test_eof734:
		(m.cs) = 734
		goto _test_eof
	_test_eof249:
		(m.cs) = 249
		goto _test_eof
	_test_eof250:
		(m.cs) = 250
		goto _test_eof
	_test_eof251:
		(m.cs) = 251
		goto _test_eof
	_test_eof735:
		(m.cs) = 735
		goto _test_eof
	_test_eof252:
		(m.cs) = 252
		goto _test_eof
	_test_eof253:
		(m.cs) = 253
		goto _test_eof
	_test_eof254:
		(m.cs) = 254
		goto _test_eof
	_test_eof736:
		(m.cs) = 736
		goto _test_eof
	_test_eof255:
		(m.cs) = 255
		goto _test_eof
	_test_eof256:
		(m.cs) = 256
		goto _test_eof
	_test_eof737:
		(m.cs) = 737
		goto _test_eof
	_test_eof738:
		(m.cs) = 738
		goto _test_eof
	_test_eof257:
		(m.cs) = 257
		goto _test_eof
	_test_eof258:
		(m.cs) = 258
		goto _test_eof
	_test_eof739:
		(m.cs) = 739
		goto _test_eof
	_test_eof261:
		(m.cs) = 261
		goto _test_eof
	_test_eof741:
		(m.cs) = 741
		goto _test_eof
	_test_eof742:
		(m.cs) = 742
		goto _test_eof
	_test_eof262:
		(m.cs) = 262
		goto _test_eof
	_test_eof263:
		(m.cs) = 263
		goto _test_eof
	_test_eof264:
		(m.cs) = 264
		goto _test_eof
	_test_eof265:
		(m.cs) = 265
		goto _test_eof
	_test_eof743:
		(m.cs) = 743
		goto _test_eof
	_test_eof266:
		(m.cs) = 266
		goto _test_eof
	_test_eof744:
		(m.cs) = 744
		goto _test_eof
	_test_eof267:
		(m.cs) = 267
		goto _test_eof
	_test_eof268:
		(m.cs) = 268
		goto _test_eof
	_test_eof269:
		(m.cs) = 269
		goto _test_eof
	_test_eof740:
		(m.cs) = 740
		goto _test_eof
	_test_eof259:
		(m.cs) = 259
		goto _test_eof
	_test_eof260:
		(m.cs) = 260
		goto _test_eof

	_test_eof:
		{
		}
		if (m.p) == (m.eof) {
			switch m.cs {
			case 8, 261:
//line plugins/parsers/influx/machine.go.rl:24

				err = ErrNameParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 2, 3, 4, 5, 6, 7, 28, 31, 32, 35, 36, 37, 49, 50, 51, 52, 53, 73, 75, 76, 93, 103, 105, 141, 153, 156, 171, 172, 173, 174, 175, 176, 177, 178, 179, 180, 181, 182, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 228, 229, 245, 246, 247, 248, 249, 250, 251, 252, 253, 254, 255, 256, 257:
//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 13, 14, 15, 22, 24, 25, 263, 264, 265, 266, 267, 268:
//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 244:
//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 741:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

			case 743, 744:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

			case 271, 272, 273, 274, 275, 277, 278, 297, 298, 299, 301, 302, 305, 306, 327, 328, 329, 330, 332, 376, 377, 379, 380, 402, 403, 408, 409, 411, 431, 432, 434, 435, 457, 458, 618, 621:
//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 10, 38, 40, 165, 167:
//line plugins/parsers/influx/machine.go.rl:24

				err = ErrNameParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 34, 74, 104, 170, 208:
//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 20, 44, 45, 46, 58, 59, 61, 63, 68, 70, 71, 77, 78, 79, 84, 86, 88, 89, 97, 98, 100, 101, 102, 107, 108, 109, 122, 123, 137, 138:
//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 60:
//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 270:
//line plugins/parsers/influx/machine.go.rl:74

				m.beginMetric = true

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 1:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 300, 303, 307, 375, 399, 400, 404, 405, 406, 530, 564, 565, 567:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 16, 23:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 351, 352, 353, 355, 374, 430, 454, 455, 459, 479, 495, 496, 498:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 624, 675, 689, 729:
//line plugins/parsers/influx/machine.go.rl:104

				err = m.handler.AddInt(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 625, 678, 692, 732:
//line plugins/parsers/influx/machine.go.rl:113

				err = m.handler.AddUint(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 326, 619, 620, 622, 623, 626, 632, 633, 671, 672, 673, 674, 676, 677, 679, 685, 686, 687, 688, 690, 691, 693, 726, 727, 728, 730, 731, 733:
//line plugins/parsers/influx/machine.go.rl:122

				err = m.handler.AddFloat(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 627, 628, 629, 630, 631, 634, 635, 636, 637, 638, 680, 681, 682, 683, 684, 734, 735, 736, 737, 738:
//line plugins/parsers/influx/machine.go.rl:131

				err = m.handler.AddBool(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 276, 279, 280, 281, 282, 283, 284, 285, 286, 287, 288, 289, 290, 291, 292, 293, 294, 295, 296, 331, 333, 334, 335, 336, 337, 338, 339, 340, 341, 342, 343, 344, 345, 346, 347, 348, 349, 350, 378, 381, 382, 383, 384, 385, 386, 387, 388, 389, 390, 391, 392, 393, 394, 395, 396, 397, 398, 410, 412, 413, 414, 415, 416, 417, 418, 419, 420, 421, 422, 423, 424, 425, 426, 427, 428, 429, 433, 436, 437, 438, 439, 440, 441, 442, 443, 444, 445, 446, 447, 448, 449, 450, 451, 452, 453, 599, 600, 601, 602, 603, 604, 605, 606, 607, 608, 609, 610, 611, 612, 613, 614, 615, 616, 617, 652, 653, 654, 655, 656, 657, 658, 659, 660, 661, 662, 663, 664, 665, 666, 667, 668, 669, 670, 707, 708, 709, 710, 711, 712, 713, 714, 715, 716, 717, 718, 719, 720, 721, 722, 723, 724, 725:
//line plugins/parsers/influx/machine.go.rl:149

				err = m.handler.SetTimestamp(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 9:
//line plugins/parsers/influx/machine.go.rl:24

				err = ErrNameParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 99:
//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 11, 12, 26, 27, 29, 30, 41, 42, 54, 55, 56, 57, 72, 91, 92, 94, 96, 139, 140, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 154, 155, 157, 158, 159, 160, 161, 162, 163, 164, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 535, 589, 697:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:104

				err = m.handler.AddInt(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 538, 592, 700:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:113

				err = m.handler.AddUint(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 407, 531, 532, 533, 534, 536, 537, 539, 563, 586, 587, 588, 590, 591, 593, 694, 695, 696, 698, 699, 701:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:122

				err = m.handler.AddFloat(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 540, 541, 542, 543, 544, 594, 595, 596, 597, 598, 702, 703, 704, 705, 706:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:131

				err = m.handler.AddBool(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 304, 308, 309, 310, 311, 312, 313, 314, 315, 316, 317, 318, 319, 320, 321, 322, 323, 324, 325, 401, 545, 546, 547, 548, 549, 550, 551, 552, 553, 554, 555, 556, 557, 558, 559, 560, 561, 562, 566, 568, 569, 570, 571, 572, 573, 574, 575, 576, 577, 578, 579, 580, 581, 582, 583, 584, 585:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:149

				err = m.handler.SetTimestamp(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 17, 18, 19, 21, 47, 48, 64, 65, 66, 67, 69, 80, 81, 82, 83, 85, 87, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 124, 125, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 194, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204, 205:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 484, 520, 642:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:104

				err = m.handler.AddInt(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 487, 523, 645:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:113

				err = m.handler.AddUint(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 478, 480, 481, 482, 483, 485, 486, 488, 494, 517, 518, 519, 521, 522, 524, 639, 640, 641, 643, 644, 646:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:122

				err = m.handler.AddFloat(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 489, 490, 491, 492, 493, 525, 526, 527, 528, 529, 647, 648, 649, 650, 651:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:131

				err = m.handler.AddBool(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 354, 356, 357, 358, 359, 360, 361, 362, 363, 364, 365, 366, 367, 368, 369, 370, 371, 372, 373, 456, 460, 461, 462, 463, 464, 465, 466, 467, 468, 469, 470, 471, 472, 473, 474, 475, 476, 477, 497, 499, 500, 501, 502, 503, 504, 505, 506, 507, 508, 509, 510, 511, 512, 513, 514, 515, 516:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:149

				err = m.handler.SetTimestamp(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:170

				m.finishMetric = true

			case 39, 166, 168, 169, 206, 207, 242, 243:
//line plugins/parsers/influx/machine.go.rl:24

				err = ErrNameParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 43, 90, 152:
//line plugins/parsers/influx/machine.go.rl:78

				err = m.handler.SetMeasurement(m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

			case 62, 106, 126:
//line plugins/parsers/influx/machine.go.rl:91

				err = m.handler.AddTag(m.key, m.text())
				if err != nil {
					(m.p)--

					(m.cs) = 258
					{
						(m.p)++
						(m.cs) = 0
						goto _out
					}
				}

//line plugins/parsers/influx/machine.go.rl:38

				err = ErrTagParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:31

				err = ErrFieldParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go.rl:45

				err = ErrTimestampParse
				(m.p)--

				(m.cs) = 258
				{
					(m.p)++
					(m.cs) = 0
					goto _out
				}

//line plugins/parsers/influx/machine.go:31897
			}
		}

	_out:
		{
		}
	}

//line plugins/parsers/influx/machine.go.rl:407

	if err != nil {
		return err
	}

	// This would indicate an error in the machine that was reported with a
	// more specific error.  We return a generic error but this should
	// possibly be a panic.
	if m.cs == 0 {
		m.cs = LineProtocol_en_discard_line
		return ErrParse
	}

	// If we haven't found a metric line yet and we reached the EOF, report it
	// now.  This happens when the data ends with a comment or whitespace.
	//
	// Otherwise we have successfully parsed a metric line, so if we are at
	// the EOF we will report it the next call.
	if !m.beginMetric && m.p == m.pe && m.pe == m.eof {
		return EOF
	}

	return nil
}

// Position returns the current byte offset into the data.
func (m *machine) Position() int {
	return m.p
}

// LineOffset returns the byte offset of the current line.
func (m *machine) LineOffset() int {
	return m.sol
}

// LineNumber returns the current line number.  Lines are counted based on the
// regular expression `\r?\n`.
func (m *machine) LineNumber() int {
	return m.lineno
}

// Column returns the current column.
func (m *machine) Column() int {
	lineOffset := m.p - m.sol
	return lineOffset + 1
}

func (m *machine) text() []byte {
	return m.data[m.pb:m.p]
}

type streamMachine struct {
	machine *machine
	reader  io.Reader
}

func NewStreamMachine(r io.Reader, handler Handler) *streamMachine {
	m := &streamMachine{
		machine: NewMachine(handler),
		reader:  r,
	}

	m.machine.SetData(make([]byte, 1024))
	m.machine.pe = 0
	m.machine.eof = -1
	return m
}

func (m *streamMachine) Next() error {
	// Check if we are already at EOF, this should only happen if called again
	// after already returning EOF.
	if m.machine.p == m.machine.pe && m.machine.pe == m.machine.eof {
		return EOF
	}

	copy(m.machine.data, m.machine.data[m.machine.p:])
	m.machine.pe = m.machine.pe - m.machine.p
	m.machine.sol = m.machine.sol - m.machine.p
	m.machine.pb = 0
	m.machine.p = 0
	m.machine.eof = -1

	m.machine.key = nil
	m.machine.beginMetric = false
	m.machine.finishMetric = false

	for {
		// Expand the buffer if it is full
		if m.machine.pe == len(m.machine.data) {
			expanded := make([]byte, 2*len(m.machine.data))
			copy(expanded, m.machine.data)
			m.machine.data = expanded
		}

		n, err := m.reader.Read(m.machine.data[m.machine.pe:])
		if n == 0 && err == io.EOF {
			m.machine.eof = m.machine.pe
		} else if err != nil && err != io.EOF {
			return err
		}

		m.machine.pe += n

		err = m.machine.exec()
		if err != nil {
			return err
		}

		// If we have successfully parsed a full metric line break out
		if m.machine.finishMetric {
			break
		}

	}

	return nil
}

// Position returns the current byte offset into the data.
func (m *streamMachine) Position() int {
	return m.machine.Position()
}

// LineOffset returns the byte offset of the current line.
func (m *streamMachine) LineOffset() int {
	return m.machine.LineOffset()
}

// LineNumber returns the current line number.  Lines are counted based on the
// regular expression `\r?\n`.
func (m *streamMachine) LineNumber() int {
	return m.machine.LineNumber()
}

// Column returns the current column.
func (m *streamMachine) Column() int {
	return m.machine.Column()
}

// LineText returns the text of the current line that has been parsed so far.
func (m *streamMachine) LineText() string {
	return string(m.machine.data[0:m.machine.p])
}
