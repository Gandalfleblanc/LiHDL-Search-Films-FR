export namespace main {
	
	export class Film {
	    tmdb_id: number;
	    title: string;
	    year: string;
	    platforms: string[];
	    resolution: string;
	    vf: string;
	
	    static createFrom(source: any = {}) {
	        return new Film(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tmdb_id = source["tmdb_id"];
	        this.title = source["title"];
	        this.year = source["year"];
	        this.platforms = source["platforms"];
	        this.resolution = source["resolution"];
	        this.vf = source["vf"];
	    }
	}
	export class GenerateResult {
	    films: Film[];
	    count: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.films = this.convertValues(source["films"], Film);
	        this.count = source["count"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    token: string;
	    useBearer: boolean;
	    platforms: string[];
	    monetize: string[];
	    criteria: string;
	    enrich: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.useBearer = source["useBearer"];
	        this.platforms = source["platforms"];
	        this.monetize = source["monetize"];
	        this.criteria = source["criteria"];
	        this.enrich = source["enrich"];
	    }
	}

}

