/// <reference path="0-bosun.ts" />

class LinkService implements ILinkService {
	public GetEditSilenceLink(silence: any, silenceId: string) : string {
		if (!(silence && silenceId)) {
			return "";
		}

		var forget = silence.Forget ? '&forget': '';
		return "/silence?start=" + this.time(silence.Start) +
			"&end=" + this.time(silence.End) +
			"&alert=" + silence.Alert +
			"&tags=" + encodeURIComponent(silence.TagString) +
			forget +
			"&edit=" + silenceId;
	}

	private time(v: any) {
		var m = moment(v).utc();
		return m.format();
	}
}

bosunApp.service("linkService", LinkService);