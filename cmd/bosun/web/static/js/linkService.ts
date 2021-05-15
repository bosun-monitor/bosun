/// <reference path="0-bosun.ts" />

class LinkService implements ILinkService {
	public GetEditSilenceLink(silence: any, silenceId: string) : string {
		if (!(silence && silenceId)) {
			return "";
		}

		var forget = silence.Forget ? '&forget': '';
		return "/silence?start=" + this.time(silence.Start) +
			"&end=" + this.time(silence.End) +
			"&periodTimeStart=" + this.timeInt2Time(silence.PeriodTimeIntStart) +
			"&periodTimeEnd=" + this.timeInt2Time(silence.PeriodTimeIntEnd) +
			"&alert=" + silence.Alert +
			"&tags=" + encodeURIComponent(silence.TagString) +
			forget +
			"&edit=" + silenceId;
	}

	private time(v: any) {
		var m = moment(v).utc();
		return m.format();
	}
	// convert 150405 to 15:04:05
	private timeInt2Time(timeInt: number) {
		var v_h = (timeInt / 10000).toFixed(0).toString();
		var v_m = this.padZero((timeInt % 10000 / 100).toFixed(0), 2);
		var v_s = this.padZero((timeInt % 100).toFixed(0), 2);
		return v_h + ":" + v_m + ":" + v_s;
	}
	private padZero(num:any, size:number): string {
		let s = num + "";
		while (s.length < size) s = "0" + s;
		return s;
	}
}

bosunApp.service("linkService", LinkService);